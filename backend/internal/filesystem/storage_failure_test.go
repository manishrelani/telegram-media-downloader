package filesystem

import (
	"errors"
	"example.com/telegram-media-downloader/backend/internal/domain"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

func TestFinalizationStorageFailureStopsLaterMoves(t *testing.T) {
	for _, mode := range []string{"journal", "move"} {
		t.Run(mode, func(t *testing.T) {
			root := t.TempDir()
			staging := filepath.Join(root, "staging")
			chat := filepath.Join(root, "chat")
			if err := os.MkdirAll(staging, 0700); err != nil {
				t.Fatal(err)
			}
			items := []domain.MediaItem{{ChatID: 123, MessageID: 3, OriginalName: "one.mp4"}, {ChatID: 123, MessageID: 2, OriginalName: "two.mp4"}, {ChatID: 123, MessageID: 1, OriginalName: "three.mp4"}}
			for _, name := range []string{"123_3_one.mp4", "123_2_two.mp4", "123_1_three.mp4"} {
				writeTestFile(t, filepath.Join(staging, name), "video bytes")
			}
			hooks := 0
			summary, err := OrganizeDownloadsWithHook(staging, chat, items, func(item domain.MediaItem, _ string) error {
				hooks++
				if item.MessageID == 2 {
					if mode == "journal" {
						return &os.PathError{Op: "write manifest", Path: chat, Err: syscall.ENOSPC}
					}
					if mode == "move" {
						if err := os.Rename(filepath.Join(chat, "Videos"), filepath.Join(chat, "kept")); err != nil {
							t.Fatal(err)
						}
						writeTestFile(t, filepath.Join(chat, "Videos"), "blocks directory")
					}
				}
				return nil
			})
			var typed *domain.AppError
			if !errors.As(err, &typed) || typed.Code != domain.ErrorCodeStorage {
				t.Fatalf("want actionable storage error, got summary=%+v err=%v", summary, err)
			}
			if summary.Renamed != 1 || summary.Failed != 1 || len(summary.Outcomes) != 2 {
				t.Fatalf("did not stop after partial success: %+v", summary)
			}
			if mode == "journal" && (!errors.Is(err, syscall.ENOSPC) || !strings.Contains(err.Error(), "manifest")) {
				t.Fatalf("lost journal cause: %v", err)
			}
			if hooks != 2 {
				t.Fatalf("later hook ran: %d", hooks)
			}
			for _, name := range []string{"123_2_two.mp4", "123_1_three.mp4"} {
				if _, err := os.Stat(filepath.Join(staging, name)); err != nil {
					t.Fatalf("recoverable source lost: %v", err)
				}
			}
		})
	}
}
