package filesystem

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"example.com/telegram-media-downloader/backend/internal/domain"
)

func TestJSONManifestStoreLoadsEmptyManifestWhenMissing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "chat", "manifest.json")
	store := JSONManifestStore{}
	got, err := store.Load(context.Background(), path, 4488394154, "GenAI")
	if err != nil {
		t.Fatal(err)
	}
	if got.Version != domain.ManifestVersion || got.ChatID != 4488394154 || got.ChatName != "GenAI" || got.Items == nil {
		t.Fatalf("new manifest = %#v", got)
	}
}

func TestJSONManifestStoreRoundTripsCompletedItems(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX permission bits are not a reliable Windows ACL assertion")
	}
	path := filepath.Join(t.TempDir(), "chat", "manifest.json")
	store := JSONManifestStore{}
	manifest := domain.NewManifest(123, "Example")
	item := domain.MediaItem{ChatID: 123, MessageID: 9, OriginalName: "lesson.mp4", Caption: "01. Lesson"}
	manifest.MarkCompleted(item, "01. Lesson.mp4")
	if err := store.Save(context.Background(), path, manifest); err != nil {
		t.Fatal(err)
	}
	loaded, err := store.Load(context.Background(), path, 123, "Example")
	if err != nil {
		t.Fatal(err)
	}
	if !loaded.IsCompleted(item) || loaded.CompletedCount() != 1 {
		t.Fatalf("loaded manifest did not preserve completion: %#v", loaded)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm()&0o077 != 0 {
		t.Fatalf("manifest permissions are too open: %o", info.Mode().Perm())
	}
}

func TestJSONManifestStoreRejectsMismatchedChatAndCorruptJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "manifest.json")
	store := JSONManifestStore{}
	if err := store.Save(context.Background(), path, domain.NewManifest(123, "Example")); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Load(context.Background(), path, 456, "Other"); err == nil {
		t.Fatal("Load accepted a manifest belonging to another chat")
	}
	if err := os.WriteFile(path, []byte("broken"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Load(context.Background(), path, 123, "Example"); err == nil {
		t.Fatal("Load accepted corrupt JSON")
	}
}

func TestJSONManifestStoreRejectsInvalidManifestAndCancelledContext(t *testing.T) {
	store := JSONManifestStore{}
	path := filepath.Join(t.TempDir(), "manifest.json")
	if err := store.Save(context.Background(), path, &domain.Manifest{Version: 999, ChatID: 123, ChatName: "Example"}); err == nil {
		t.Fatal("Save accepted unsupported manifest version")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := store.Load(ctx, path, 123, "Example"); !errors.Is(err, context.Canceled) {
		t.Fatalf("Load error = %v, want context.Canceled", err)
	}
	if err := store.Save(ctx, path, domain.NewManifest(123, "Example")); !errors.Is(err, context.Canceled) {
		t.Fatalf("Save error = %v, want context.Canceled", err)
	}
}
