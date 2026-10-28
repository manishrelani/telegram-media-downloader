package filesystem

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"example.com/telegram-media-downloader/backend/internal/domain"
)

func TestOrganizeDownloadsUsesCaptionsAndCategorizesFiles(t *testing.T) {
	root := t.TempDir()
	staging := filepath.Join(root, ".staging")
	chatRoot := filepath.Join(root, "ChaiCode GenAI")
	if err := os.MkdirAll(staging, 0o700); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(staging, "123_26_lesson.mp4"), "video")
	writeTestFile(t, filepath.Join(staging, "123_25_notes.pdf"), "pdf")
	writeTestFile(t, filepath.Join(staging, "123_24_unknown.xyz"), "other")

	items := []domain.MediaItem{
		{ChatID: 123, MessageID: 26, Caption: "01. Introduction to GenAI", OriginalName: "lesson.mp4"},
		{ChatID: 123, MessageID: 25, Caption: "RAG Architecture Notes", OriginalName: "notes.pdf"},
		{ChatID: 123, MessageID: 24, Caption: "Supporting Archive", OriginalName: "unknown.xyz"},
	}
	result, err := OrganizeDownloads(staging, chatRoot, items)
	if err != nil {
		t.Fatal(err)
	}
	if result.Renamed != 3 || result.Skipped != 0 || result.Failed != 0 {
		t.Fatalf("summary = %+v", result)
	}
	for _, path := range []string{
		filepath.Join(chatRoot, "Videos", "01. Introduction to GenAI.mp4"),
		filepath.Join(chatRoot, "Documents", "RAG Architecture Notes.pdf"),
		filepath.Join(chatRoot, "Attachments", "Supporting Archive.xyz"),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Errorf("expected output %q: %v", path, err)
		}
	}
}

func TestOrganizeDownloadsFallsBackToOriginalNameForEmptyCaption(t *testing.T) {
	root := t.TempDir()
	staging := filepath.Join(root, ".staging")
	chatRoot := filepath.Join(root, "Example")
	if err := os.MkdirAll(staging, 0o700); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(staging, "123_7_original-name.mp4"), "video")
	result, err := OrganizeDownloads(staging, chatRoot, []domain.MediaItem{{ChatID: 123, MessageID: 7, OriginalName: "original-name.mp4"}})
	if err != nil {
		t.Fatal(err)
	}
	if result.Renamed != 1 {
		t.Fatalf("summary = %+v", result)
	}
	if _, err := os.Stat(filepath.Join(chatRoot, "Videos", "original-name.mp4")); err != nil {
		t.Fatal(err)
	}
}

func TestOrganizeDownloadsAddsSuffixInsteadOfOverwriting(t *testing.T) {
	root := t.TempDir()
	staging := filepath.Join(root, ".staging")
	chatRoot := filepath.Join(root, "Example")
	if err := os.MkdirAll(staging, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(chatRoot, "Videos"), 0o700); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(chatRoot, "Videos", "Lecture.mp4"), "previous")
	writeTestFile(t, filepath.Join(staging, "123_8_lesson.mp4"), "new")
	result, err := OrganizeDownloads(staging, chatRoot, []domain.MediaItem{{ChatID: 123, MessageID: 8, Caption: "Lecture", OriginalName: "lesson.mp4"}})
	if err != nil {
		t.Fatal(err)
	}
	if result.Renamed != 1 {
		t.Fatalf("summary = %+v", result)
	}
	if got := readTestFile(t, filepath.Join(chatRoot, "Videos", "Lecture.mp4")); got != "previous" {
		t.Fatalf("existing file overwritten: %q", got)
	}
	if got := readTestFile(t, filepath.Join(chatRoot, "Videos", "Lecture (2).mp4")); got != "new" {
		t.Fatalf("new file = %q", got)
	}
}

func TestOrganizeDownloadsReportsMissingAndAmbiguousFiles(t *testing.T) {
	root := t.TempDir()
	staging := filepath.Join(root, ".staging")
	chatRoot := filepath.Join(root, "Example")
	if err := os.MkdirAll(staging, 0o700); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(staging, "123_9_a.mp4"), "one")
	writeTestFile(t, filepath.Join(staging, "123_9_b.mp4"), "two")
	items := []domain.MediaItem{
		{ChatID: 123, MessageID: 99, Caption: "Missing", OriginalName: "miss.mp4"},
		{ChatID: 123, MessageID: 9, Caption: "Ambiguous", OriginalName: "different.mp4"},
	}
	result, err := OrganizeDownloads(staging, chatRoot, items)
	if err != nil {
		t.Fatal(err)
	}
	if result.Skipped != 1 || result.Failed != 1 {
		t.Fatalf("summary = %+v", result)
	}
}

func TestOrganizeDownloadsPersistsPlanBeforeMoving(t *testing.T) {
	root := t.TempDir()
	staging := filepath.Join(root, ".staging")
	chatRoot := filepath.Join(root, "Example")
	if err := os.MkdirAll(staging, 0o700); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(staging, "123_11_lesson.mp4")
	writeTestFile(t, source, "video bytes")
	called := false
	result, err := OrganizeDownloadsWithHook(staging, chatRoot, []domain.MediaItem{{ChatID: 123, MessageID: 11, Caption: "Lesson", OriginalName: "lesson.mp4"}}, func(item domain.MediaItem, plannedName string) error {
		called = true
		if item.MessageID != 11 || plannedName != "Lesson.mp4" {
			t.Fatalf("hook args = %+v, %q", item, plannedName)
		}
		if _, err := os.Stat(source); err != nil {
			t.Fatalf("source should exist before move: %v", err)
		}
		if _, err := os.Stat(filepath.Join(chatRoot, "Videos", plannedName)); !os.IsNotExist(err) {
			t.Fatalf("destination should not exist before move, stat err=%v", err)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if !called || result.Renamed != 1 {
		t.Fatalf("hook=%v result=%+v", called, result)
	}
}

func TestOrganizeDownloadsDoesNotMoveWhenPreMoveHookFails(t *testing.T) {
	root := t.TempDir()
	staging := filepath.Join(root, ".staging")
	chatRoot := filepath.Join(root, "Example")
	if err := os.MkdirAll(staging, 0o700); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(staging, "123_12_lesson.mp4")
	writeTestFile(t, source, "video bytes")
	result, err := OrganizeDownloadsWithHook(staging, chatRoot, []domain.MediaItem{{ChatID: 123, MessageID: 12, Caption: "Lesson", OriginalName: "lesson.mp4"}}, func(domain.MediaItem, string) error {
		return os.ErrPermission
	})
	if !errors.Is(err, os.ErrPermission) {
		t.Fatalf("lost storage cause: %v", err)
	}
	if result.Failed != 1 || result.Renamed != 0 {
		t.Fatalf("summary = %+v", result)
	}
	if _, err := os.Stat(source); err != nil {
		t.Fatalf("staging file was moved despite hook failure: %v", err)
	}
	if _, err := os.Stat(filepath.Join(chatRoot, "Videos", "Lesson.mp4")); !os.IsNotExist(err) {
		t.Fatalf("unexpected destination exists, stat err=%v", err)
	}
}

func TestOrganizeDownloadsRejectsEmptyPaths(t *testing.T) {
	if _, err := OrganizeDownloads("", "/tmp/out", nil); err == nil {
		t.Fatal("expected path error")
	}
	if _, err := OrganizeDownloads("/tmp/staging", "", nil); err == nil {
		t.Fatal("expected path error")
	}
}

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func readTestFile(t *testing.T, path string) string {
	t.Helper()
	bytes, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(bytes)
}
