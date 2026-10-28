package domain

import "testing"

func TestClassifyMedia(t *testing.T) {
	tests := []struct {
		name string
		want MediaCategory
	}{
		{"lecture.MP4", CategoryVideo},
		{"clip.webm", CategoryVideo},
		{"notes.pdf", CategoryDocument},
		{"slides.PPTX", CategoryDocument},
		{"readme.md", CategoryDocument},
		{"photo.jpg", CategoryOther},
		{"archive.unknown", CategoryOther},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ClassifyMedia(tt.name); got != tt.want {
				t.Fatalf("ClassifyMedia(%q) = %q, want %q", tt.name, got, tt.want)
			}
		})
	}
}

func TestShouldIncludeMedia(t *testing.T) {
	tests := []struct {
		name string
		mode FilterMode
		want bool
	}{
		{"lecture.mp4", FilterVideosAndDocuments, true},
		{"notes.pdf", FilterVideosAndDocuments, true},
		{"photo.jpg", FilterVideosAndDocuments, false},
		{"lecture.mp4", FilterVideosOnly, true},
		{"notes.pdf", FilterVideosOnly, false},
		{"notes.pdf", FilterDocumentsOnly, true},
		{"lecture.mp4", FilterDocumentsOnly, false},
		{"archive.unknown", FilterAllAttachments, true},
	}
	for _, tt := range tests {
		t.Run(tt.name+"_"+string(tt.mode), func(t *testing.T) {
			got, err := ShouldIncludeMedia(tt.name, tt.mode)
			if err != nil {
				t.Fatalf("ShouldIncludeMedia() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("ShouldIncludeMedia(%q, %q) = %v, want %v", tt.name, tt.mode, got, tt.want)
			}
		})
	}
}

func TestShouldIncludeMediaRejectsUnknownMode(t *testing.T) {
	if _, err := ShouldIncludeMedia("a.mp4", FilterMode("everything-ish")); err == nil {
		t.Fatal("ShouldIncludeMedia() error = nil, want unsupported filter error")
	}
}
