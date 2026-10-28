package application

import (
	"context"
	"errors"
	"example.com/telegram-media-downloader/backend/internal/domain"
	"example.com/telegram-media-downloader/backend/internal/filesystem"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

type fullJournalStore struct {
	filesystem.JSONManifestStore
	fail bool
}

func (m *fullJournalStore) Save(ctx context.Context, path string, manifest *domain.Manifest) error {
	if m.fail {
		for _, entry := range manifest.Items {
			if entry.MessageID == 2 && entry.Status == "moving" {
				return &os.PathError{Op: "write manifest", Path: path, Err: syscall.ENOSPC}
			}
		}
	}
	return m.JSONManifestStore.Save(ctx, path, manifest)
}
func TestStorageFinalizationStopsBatchAndRetryKeepsSavedFiles(t *testing.T) {
	gateway := newFakeGateway([]fakeMedia{{id: 3, name: "saved.mp4"}, {id: 2, name: "blocked.mp4"}, {id: 1, name: "later.mp4"}})
	s, jobs := newTestService(t, gateway)
	s.Telegram = &libraryGateway{fakeGateway: gateway}
	journal := &fullJournalStore{fail: true}
	s.Manifests = journal
	result, err := s.RunMedia(context.Background(), MediaRunRequest{JobID: "disk-full", Chat: domain.Chat{ID: 123, Name: "Course"}, DestinationRoot: t.TempDir(), Selection: domain.MediaSelection{EntireScope: true, FileTypes: []string{"video"}}}, nil)
	if !errors.Is(err, syscall.ENOSPC) || result.Job.ErrorCode != string(domain.ErrorCodeStorage) || !strings.Contains(result.Job.ErrorMessage, "manifest") {
		t.Fatalf("lost storage failure: job=%+v err=%v", result.Job, err)
	}
	if gateway.downloadCalls != 2 || result.Job.Files[0].State != "completed" || result.Job.Files[1].State != "failed" || result.Job.Files[2].State != "queued" {
		t.Fatalf("later files ran: calls=%d files=%+v", gateway.downloadCalls, result.Job.Files)
	}
	saved := result.Job.Files[0].SavedPath
	if data, err := os.ReadFile(saved); err != nil || string(data) != "test media" {
		t.Fatalf("saved file lost: %q %v", data, err)
	}
	partial := filepath.Join(mediaWorkDir(result.Job), workDirectoryName("123:2:blocked.mp4"), "staging", "123_2_blocked.mp4")
	if _, err := os.Stat(partial); err != nil {
		t.Fatalf("partial file lost: %v", err)
	}
	journal.fail = false
	fresh := NewService(s.Telegram, jobs, s.Organizer, journal)
	retried, err := fresh.RetryMedia(context.Background(), result.Job.ID, nil)
	if err != nil || retried.Job.State != domain.JobCompleted || gateway.downloadCalls != 4 {
		t.Fatalf("retry failed or re-downloaded a finalized file: calls=%d state=%s err=%v", gateway.downloadCalls, retried.Job.State, err)
	}
	if retried.Job.Files[0].SavedPath != saved {
		t.Fatal("saved file path changed")
	}
}

func TestLegacyStorageFailureReportsSavedPartialOutcomes(t *testing.T) {
	gateway := newFakeGateway([]fakeMedia{{id: 3, name: "saved.mp4"}, {id: 2, name: "blocked.mp4"}, {id: 1, name: "later.mp4"}})
	s, _ := newTestService(t, gateway)
	s.Manifests = &fullJournalStore{fail: true}
	var terminal *domain.OrganizationSummary
	result, err := s.Run(context.Background(), RunRequest{JobID: "legacy-full", Chat: domain.Chat{ID: 123, Name: "Course"}, DestinationRoot: t.TempDir(), SkipSame: true}, func(event Event) {
		if event.Kind == "job_failed" {
			terminal = event.Summary
		}
	})
	if !errors.Is(err, syscall.ENOSPC) || result.Job.ErrorCode != string(domain.ErrorCodeStorage) {
		t.Fatalf("lost storage error: %v", err)
	}
	if result.Summary.Renamed != 1 || result.Summary.Failed != 1 || len(result.Summary.Outcomes) != 2 || terminal == nil || terminal.Renamed != 1 {
		t.Fatalf("partial outcomes lost: result=%+v terminal=%+v", result.Summary, terminal)
	}
	if _, err := os.Stat(result.Summary.Outcomes[0].SavedPath); err != nil {
		t.Fatalf("saved item lost: %v", err)
	}
}
func TestLegacyOrganizerSetupFailureIsStorageError(t *testing.T) {
	gateway := newFakeGateway([]fakeMedia{{id: 7, name: "saved.mp4"}})
	s, _ := newTestService(t, gateway)
	root := t.TempDir()
	chatRoot := filepath.Join(root, "Course [123]")
	if err := os.MkdirAll(chatRoot, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(chatRoot, "Documents"), []byte("blocks folder"), 0600); err != nil {
		t.Fatal(err)
	}
	result, err := s.Run(context.Background(), RunRequest{JobID: "legacy-blocked-directory", Chat: domain.Chat{ID: 123, Name: "Course"}, DestinationRoot: root, SkipSame: true}, nil)
	if err == nil || result.Job.ErrorCode != string(domain.ErrorCodeStorage) || !strings.Contains(result.Job.ErrorMessage, "directory") {
		t.Fatalf("setup storage failure misclassified: code=%s err=%v", result.Job.ErrorCode, err)
	}
}
