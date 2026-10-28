package application

import (
	"context"
	"example.com/telegram-media-downloader/backend/internal/domain"
	"os"
	"path/filepath"
	"testing"
)

func TestPrivateWorkMigrationPreservesInputAndRejectsLinks(t *testing.T) {
	for _, link := range []bool{false, true} {
		t.Run(map[bool]string{false: "regular", true: "symlink"}[link], func(t *testing.T) {
			s, jobs := newTestService(t, newFakeGateway(nil))
			root := t.TempDir()
			job := domain.DownloadJob{ID: "old", DestinationRoot: root, Destination: filepath.Join(root, "chat"), State: domain.JobPaused, Filter: domain.FilterAllAttachments}
			old := mediaWorkDir(job)
			if err := os.MkdirAll(old, 0700); err != nil {
				t.Fatal(err)
			}
			content := []byte("private recovery input")
			input := filepath.Join(old, "export.json")
			if link {
				outside := filepath.Join(t.TempDir(), "private.json")
				if err := os.WriteFile(outside, content, 0600); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(outside, input); err != nil {
					t.Skip(err)
				}
			} else if err := os.WriteFile(input, content, 0600); err != nil {
				t.Fatal(err)
			}
			if err := jobs.Save(context.Background(), job); err != nil {
				t.Fatal(err)
			}
			err := s.ensurePrivateWork(context.Background(), &job)
			if link {
				if err == nil || job.WorkDirectory != "" {
					t.Fatal("linked legacy input accepted")
				}
			} else {
				if err != nil {
					t.Fatal(err)
				}
				got, err := os.ReadFile(filepath.Join(job.WorkDirectory, "export.json"))
				if err != nil || string(got) != string(content) {
					t.Fatalf("migration lost recovery data: %q %v", got, err)
				}
				if _, err := os.Stat(input); err != nil {
					t.Fatal("old recovery data removed")
				}
			}
		})
	}
}
