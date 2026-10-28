package filesystem

import (
	"context"
	"example.com/telegram-media-downloader/backend/internal/domain"
	"os"
	"path/filepath"
	"testing"
)

func TestSecurityRejectsCategoryReplacedDuringJournal(t *testing.T) {
	root := t.TempDir()
	staging := filepath.Join(root, "chosen", "staging")
	chat := filepath.Join(root, "chosen", "chat")
	outside := filepath.Join(root, "outside")
	os.MkdirAll(staging, 0700)
	os.MkdirAll(outside, 0700)
	os.MkdirAll(chat, 0700)
	os.WriteFile(filepath.Join(staging, "123_7_lesson.mp4"), []byte("harmless"), 0600)
	_, err := OrganizeDownloadsWithHook(staging, chat, []domain.MediaItem{{ChatID: 123, MessageID: 7, OriginalName: "lesson.mp4"}}, func(domain.MediaItem, string) error {
		if err := os.Rename(filepath.Join(chat, "Videos"), filepath.Join(chat, "old-videos")); err != nil {
			return err
		}
		return os.Symlink(outside, filepath.Join(chat, "Videos"))
	})
	if err == nil {
		t.Fatal("journal/move race followed a replaced output directory")
	}
	if _, err := os.Stat(filepath.Join(outside, "lesson.mp4")); !os.IsNotExist(err) {
		t.Fatal("escaped output created")
	}
}
func TestSecurityManifestWriteRejectsSymlinkChat(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	os.Symlink(outside, filepath.Join(root, "chat"))
	err := (JSONManifestStore{}).Save(context.Background(), filepath.Join(root, "chat", ".telegram-media-manager-manifest.json"), domain.NewManifest(123, "Course"))
	if err == nil {
		t.Fatal("manifest write followed an escaping chat symlink")
	}
	if _, err := os.Stat(filepath.Join(outside, ".telegram-media-manager-manifest.json")); !os.IsNotExist(err) {
		t.Fatal("manifest escaped root")
	}
}
