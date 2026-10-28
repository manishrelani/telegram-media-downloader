package domain

import (
	"testing"
	"unicode/utf8"
)

func TestFilenameForUsesCaptionAndPreservesExtension(t *testing.T) {
	got := FilenameFor("01. Introduction to GenAI", "4488394154_4_0000_Introduction.mp4")
	if got != "01. Introduction to GenAI.mp4" {
		t.Fatalf("FilenameFor() = %q, want %q", got, "01. Introduction to GenAI.mp4")
	}
}

func TestFilenameForFallsBackToOriginalNameWhenCaptionEmpty(t *testing.T) {
	got := FilenameFor(" \n\t ", "4488394154_4_0000_Introduction.mp4")
	if got != "4488394154_4_0000_Introduction.mp4" {
		t.Fatalf("FilenameFor() = %q, want original filename", got)
	}
}

func TestFilenameForDoesNotDuplicateMatchingExtension(t *testing.T) {
	got := FilenameFor("RAG Notes.pdf", "12_99_rag-notes.PDF")
	if got != "RAG Notes.PDF" {
		t.Fatalf("FilenameFor() = %q, want %q", got, "RAG Notes.PDF")
	}
}

func TestFilenameForSanitizesUnsafePathAndPlatformCharacters(t *testing.T) {
	got := FilenameFor("../Notes: <private> / draft?", "notes.pdf")
	if got != "-Notes- -private- - draft-.pdf" {
		t.Fatalf("FilenameFor() = %q, want sanitized filename", got)
	}
	if got == ".." || got == "." {
		t.Fatalf("unsafe filename: %q", got)
	}
}

func TestFilenameForPreservesUnicodeEmojiAndNonLatin(t *testing.T) {
	got := FilenameFor("生成AI 🚀 परिचय", "video.mp4")
	if got != "生成AI 🚀 परिचय.mp4" {
		t.Fatalf("FilenameFor() = %q", got)
	}
}

func TestFilenameForSanitizesReservedWindowsName(t *testing.T) {
	got := FilenameFor("CON", "original.pdf")
	if got != "_CON.pdf" {
		t.Fatalf("FilenameFor() = %q, want _CON.pdf", got)
	}
}

func TestFilenameForTruncatesLongCaptionWithoutBreakingUTF8(t *testing.T) {
	caption := ""
	for i := 0; i < 200; i++ {
		caption += "界"
	}
	got := FilenameFor(caption, "video.mp4")
	if len(got) > 200 {
		t.Fatalf("filename is too long: %d bytes", len(got))
	}
	if !isValidUTF8(got) {
		t.Fatalf("filename is not valid UTF-8")
	}
}

func TestFilenameAllocatorAvoidsExistingAndRepeatedNames(t *testing.T) {
	allocator := NewFilenameAllocator([]string{"lecture.MP4"})
	got1 := allocator.Allocate("Lecture", "one.mp4")
	got2 := allocator.Allocate("Lecture", "two.mp4")
	got3 := allocator.Allocate("Lecture", "three.mp4")
	want := []string{"Lecture (2).mp4", "Lecture (3).mp4", "Lecture (4).mp4"}
	got := []string{got1, got2, got3}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("got[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func isValidUTF8(value string) bool {
	return utf8.ValidString(value)
}
