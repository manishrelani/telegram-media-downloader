package domain

import "testing"

func TestFileTypeSelection(t *testing.T) {
	for _, tc := range []struct{ name, kind string }{
		{"lesson.mp4", "video"}, {"notes.PDF", "pdf"}, {"slides.pptx", "office"},
		{"readme.md", "text"}, {"book.epub", "ebook"}, {"source.tar.gz", "archive"},
		{"index.ts", "code"}, {"recording.mp3", "audio"},
	} {
		if got := FileType(tc.name); got != tc.kind {
			t.Errorf("%s: got %s, want %s", tc.name, got, tc.kind)
		}
		if !MatchesFileTypes(tc.name, []string{tc.kind}) {
			t.Errorf("selection excluded %s", tc.name)
		}
	}
	if MatchesFileTypes("notes.pdf", []string{"video"}) {
		t.Fatal("video selection included PDF")
	}
	if !MatchesFileTypes("unknown.xyz", []string{"all"}) {
		t.Fatal("all must include unknown extensions")
	}
	if ValidateFileTypes([]string{}) == nil || ValidateFileTypes([]string{"typo"}) == nil {
		t.Fatal("empty/unknown filters must fail closed")
	}
}
