package application

import (
	"context"
	"example.com/telegram-media-downloader/backend/internal/domain"
	"os"
	"path/filepath"
	"testing"
)

func TestSecurityManifestNamesAndPaths(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "Videos"), 0700)
	os.WriteFile(filepath.Join(root, "Videos", "lesson.mp4"), []byte("video"), 0600)
	item := domain.MediaItem{ChatID: 123, MessageID: 7, OriginalName: "lesson.mp4"}
	for _, test := range []struct{ name, path string }{
		{"../../other.mp4", ""}, {"..\\..\\other.mp4", ""}, {"/tmp/other.mp4", ""}, {".", ""}, {"lesson.command", ""}, {"lesson.mp4", "../other.mp4"}, {"lesson.mp4", "..\\other.mp4"}, {"lesson.mp4", "Videos/other.command"}, {"lesson.mp4", "Videos/different.mp4"},
	} {
		t.Run(test.name+test.path, func(t *testing.T) {
			_, err := completedFilePresent(root, item, domain.ManifestEntry{SavedName: test.name, SavedPath: test.path})
			if err == nil {
				t.Fatal("unsafe manifest path accepted")
			}
		})
	}
	present, err := completedFilePresent(root, item, domain.ManifestEntry{SavedName: "lesson.mp4"})
	if err != nil || !present {
		t.Fatalf("valid legacy entry rejected: %v", err)
	}
}
func TestSecurityValidateBeforeOpen(t *testing.T) {
	root := t.TempDir()
	chat := filepath.Join(root, "Course [123]")
	os.MkdirAll(filepath.Join(chat, "Videos"), 0700)
	valid := filepath.Join(chat, "Videos", "lesson.mp4")
	os.WriteFile(valid, []byte("video"), 0600)
	outside := filepath.Join(t.TempDir(), "harmless.command")
	os.WriteFile(outside, []byte("harmless"), 0600)
	job := domain.DownloadJob{DestinationRoot: root, Destination: chat}
	for _, path := range []string{outside, filepath.Join(chat, "Videos", "missing.mp4")} {
		if _, err := ValidateSavedDownload(job, domain.FileTask{Item: domain.MediaItem{OriginalName: "lesson.mp4"}, SavedPath: path, State: "completed"}); err == nil {
			t.Fatalf("unsafe path accepted: %s", path)
		}
	}
	link := filepath.Join(chat, "Videos", "linked.mp4")
	os.Symlink(outside, link)
	if _, err := ValidateSavedDownload(job, domain.FileTask{Item: domain.MediaItem{OriginalName: "lesson.mp4"}, SavedPath: link, State: "completed"}); err == nil {
		t.Fatal("symlink accepted before Open")
	}
	if _, err := ValidateSavedDownload(job, domain.FileTask{Item: domain.MediaItem{OriginalName: "lesson.mp4"}, SavedPath: valid, State: "completed"}); err != nil {
		t.Fatalf("valid file rejected: %v", err)
	}
}

func TestRecoveryUsesValidatedPortableManifestPath(t *testing.T) {
	s, _ := newTestService(t, newFakeGateway(nil))
	root := t.TempDir()
	chat := filepath.Join(root, "chat")
	if err := os.MkdirAll(filepath.Join(chat, "Videos"), 0700); err != nil {
		t.Fatal(err)
	}
	saved := filepath.Join(chat, "Videos", "lesson.mp4")
	if err := os.WriteFile(saved, []byte("video"), 0600); err != nil {
		t.Fatal(err)
	}
	item := domain.MediaItem{ChatID: 123, MessageID: 7, OriginalName: "lesson.mp4"}
	manifest := domain.NewManifest(123, "chat")
	manifest.MarkCompleted(item, "lesson.mp4")
	entry := manifest.Items[item.Key()]
	entry.SavedPath = `Videos\lesson.mp4`
	manifest.Items[item.Key()] = entry
	if err := s.Manifests.Save(context.Background(), filepath.Join(chat, ".telegram-media-manager-manifest.json"), manifest); err != nil {
		t.Fatal(err)
	}
	job := domain.DownloadJob{ID: "portable", State: domain.JobDownloading, Filter: domain.FilterAllAttachments, Destination: chat, DestinationRoot: root, Chat: domain.Chat{ID: 123}, Files: []domain.FileTask{{Item: item, State: "queued"}}}
	if err := s.transferFile(context.Background(), &job, 0, nil, nil); err != nil {
		t.Fatal(err)
	}
	if job.Files[0].SavedPath != saved {
		t.Fatalf("recovered path %q, want %q", job.Files[0].SavedPath, saved)
	}
	if _, err := ValidateSavedDownload(job, job.Files[0]); err != nil {
		t.Fatal(err)
	}
}
