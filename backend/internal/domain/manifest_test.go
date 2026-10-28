package domain

import "testing"

func TestManifestTracksCompletedMediaByStableKey(t *testing.T) {
	manifest := NewManifest(123, "Example Channel")
	item := MediaItem{ChatID: 123, MessageID: 42, OriginalName: "lesson.mp4", Caption: "Lesson 1"}
	if manifest.IsCompleted(item) {
		t.Fatal("new manifest item unexpectedly completed")
	}
	manifest.MarkCompleted(item, "Lesson 1.mp4")
	if !manifest.IsCompleted(item) {
		t.Fatal("completed item was not found in manifest")
	}
	if manifest.CompletedCount() != 1 {
		t.Fatalf("CompletedCount() = %d, want 1", manifest.CompletedCount())
	}
}

func TestManifestDoesNotConfuseItemsWithSameMessageIDButDifferentFiles(t *testing.T) {
	manifest := NewManifest(123, "Example Channel")
	one := MediaItem{ChatID: 123, MessageID: 42, OriginalName: "lesson.mp4"}
	two := MediaItem{ChatID: 123, MessageID: 42, OriginalName: "slides.pdf"}
	manifest.MarkCompleted(one, "Lesson.mp4")
	if manifest.IsCompleted(two) {
		t.Fatal("different media from same message incorrectly treated as completed")
	}
}
