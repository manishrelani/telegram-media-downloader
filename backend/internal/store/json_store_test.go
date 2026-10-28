package store

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"example.com/telegram-media-downloader/backend/internal/domain"
)

func TestJSONJobStorePersistsAcrossRestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "jobs.json")
	first, err := NewJSONJobStore(path)
	if err != nil {
		t.Fatal(err)
	}
	job := validJob("job-1")
	job.Items = []domain.MediaItem{{ChatID: 123, MessageID: 9, OriginalName: "lesson.mp4", Caption: "Lesson"}}
	if err := first.Save(context.Background(), job); err != nil {
		t.Fatal(err)
	}

	second, err := NewJSONJobStore(path)
	if err != nil {
		t.Fatal(err)
	}
	got, err := second.Get(context.Background(), "job-1")
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != job.ID || got.State != job.State || got.Items[0].Caption != "Lesson" {
		t.Fatalf("loaded job = %+v", got)
	}
}

func TestJSONJobStoreCopiesNestedFileProgressAndTopicSelection(t *testing.T) {
	s, err := NewJSONJobStore(filepath.Join(t.TempDir(), "jobs.json"))
	if err != nil {
		t.Fatal(err)
	}
	job := validJob("library")
	percent := 42.0
	topic := &domain.Topic{ID: 42, Title: "Backend"}
	job.Library = true
	job.Topic = topic
	job.Selection = &domain.MediaSelection{Topic: topic, FileTypes: []string{"pdf"}}
	job.Files = []domain.FileTask{{Item: domain.MediaItem{ChatID: 123, MessageID: 2, OriginalName: "notes.pdf"}, State: "downloading", Progress: &domain.ProgressEvent{Percent: &percent}}}
	if err := s.Save(context.Background(), job); err != nil {
		t.Fatal(err)
	}
	*job.Files[0].Progress.Percent = 90
	job.Selection.Topic.Title = "mutated"
	stored, err := s.Get(context.Background(), job.ID)
	if err != nil {
		t.Fatal(err)
	}
	if *stored.Files[0].Progress.Percent != 42 || stored.Selection.Topic.Title != "Backend" {
		t.Fatalf("caller mutated stored job: %+v", stored)
	}
}

func TestJSONJobStoreListIsDeterministicAndCopiesItems(t *testing.T) {
	store, err := NewJSONJobStore(filepath.Join(t.TempDir(), "jobs.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"job-c", "job-a", "job-b"} {
		if err := store.Save(context.Background(), validJob(id)); err != nil {
			t.Fatal(err)
		}
	}
	jobs, err := store.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(jobs) != 3 || jobs[0].ID != "job-a" || jobs[1].ID != "job-b" || jobs[2].ID != "job-c" {
		t.Fatalf("jobs not sorted deterministically: %#v", jobs)
	}
	jobs[0].Items = append(jobs[0].Items, domain.MediaItem{OriginalName: "mutated.pdf"})
	reloaded, err := store.Get(context.Background(), "job-a")
	if err != nil {
		t.Fatal(err)
	}
	if len(reloaded.Items) != 0 {
		t.Fatalf("returned slice mutated store state: %#v", reloaded.Items)
	}
}

func TestJSONJobStoreDeleteAndNotFound(t *testing.T) {
	store, err := NewJSONJobStore(filepath.Join(t.TempDir(), "jobs.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Save(context.Background(), validJob("job-1")); err != nil {
		t.Fatal(err)
	}
	if err := store.Delete(context.Background(), "job-1"); err != nil {
		t.Fatal(err)
	}
	_, err = store.Get(context.Background(), "job-1")
	if !errors.Is(err, ErrJobNotFound) {
		t.Fatalf("Get error = %v, want ErrJobNotFound", err)
	}
	if err := store.Delete(context.Background(), "missing"); !errors.Is(err, ErrJobNotFound) {
		t.Fatalf("Delete error = %v", err)
	}
}

func TestJSONJobStoreRejectsInvalidJob(t *testing.T) {
	store, err := NewJSONJobStore(filepath.Join(t.TempDir(), "jobs.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, job := range []domain.DownloadJob{{State: domain.JobQueued, Destination: "/tmp/out"}, {ID: "invalid", State: "banana", Destination: "/tmp/out"}, {ID: "no-destination", State: domain.JobQueued}} {
		if err := store.Save(context.Background(), job); err == nil {
			t.Fatalf("Save accepted invalid job: %+v", job)
		}
	}
}

func TestJSONJobStoreRespectsCancelledContext(t *testing.T) {
	store, err := NewJSONJobStore(filepath.Join(t.TempDir(), "jobs.json"))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := store.Save(ctx, validJob("job-1")); !errors.Is(err, context.Canceled) {
		t.Fatalf("Save error = %v, want context.Canceled", err)
	}
}

func TestJSONJobStoreRejectsCorruptStateFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "jobs.json")
	if err := os.WriteFile(path, []byte("not-json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := NewJSONJobStore(path); err == nil {
		t.Fatal("NewJSONJobStore accepted malformed JSON")
	}
}

func TestJSONJobStoreFileIsPrivate(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Go file-mode bits do not represent Windows ACL permissions")
	}
	path := filepath.Join(t.TempDir(), "jobs.json")
	store, err := NewJSONJobStore(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Save(context.Background(), validJob("job-1")); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm()&0o077 != 0 {
		t.Fatalf("state file permissions are too open: %o", info.Mode().Perm())
	}
}

func validJob(id string) domain.DownloadJob {
	return domain.DownloadJob{ID: id, Chat: domain.Chat{ID: 123, Type: "channel", Name: "Example"}, State: domain.JobQueued, Destination: "/tmp/out", Filter: domain.FilterVideosAndDocuments}
}
