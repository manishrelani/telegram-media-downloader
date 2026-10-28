package application

import (
	"context"
	"example.com/telegram-media-downloader/backend/internal/domain"
	"os"
	"path/filepath"
	"testing"
)

func TestSavedFileAvailabilityDoesNotMutateDownloadHistory(t *testing.T) {
	s, jobs := newTestService(t, newFakeGateway(nil))
	root := t.TempDir()
	existing := filepath.Join(root, "saved.mp4")
	if err := os.WriteFile(existing, []byte("saved"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ id, path, root, want string }{
		{"present", existing, root, "available"}, {"missing", filepath.Join(root, "gone.mp4"), root, "missing"}, {"offline", filepath.Join(root, "offline", "gone.mp4"), filepath.Join(root, "offline"), "unavailable"}, {"legacy", "", root, "unknown"},
	} {
		t.Run(tc.id, func(t *testing.T) {
			job := domain.DownloadJob{ID: tc.id, Chat: domain.Chat{ID: 1}, State: domain.JobCompleted, Filter: domain.FilterAllAttachments, Destination: tc.root, DestinationRoot: tc.root, Files: []domain.FileTask{{Item: domain.MediaItem{ChatID: 1, MessageID: 1, OriginalName: "saved.mp4"}, State: "completed", SavedPath: tc.path}}}
			if err := jobs.Save(context.Background(), job); err != nil {
				t.Fatal(err)
			}
			listed, err := s.ListJobs(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			found := false
			for _, j := range listed {
				if j.ID == tc.id {
					found = true
					if j.Files[0].Availability != tc.want || j.State != domain.JobCompleted {
						t.Fatalf("job=%+v", j)
					}
				}
			}
			if !found {
				t.Fatal("missing job")
			}
			stored, _ := jobs.Get(context.Background(), tc.id)
			if stored.Files[0].Availability != "" {
				t.Fatal("ephemeral file availability persisted")
			}
		})
	}
}
