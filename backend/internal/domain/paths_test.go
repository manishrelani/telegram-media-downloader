package domain

import (
	"path/filepath"
	"testing"
)

func TestBuildChatDirectoriesCreatesExpectedLayout(t *testing.T) {
	got, err := BuildChatDirectories("/tmp/downloads", "ChaiCode GenAI")
	if err != nil {
		t.Fatalf("BuildChatDirectories() error = %v", err)
	}
	wantRoot := filepath.Join("/tmp/downloads", "ChaiCode GenAI")
	if got.Root != wantRoot || got.Videos != filepath.Join(wantRoot, "Videos") || got.Documents != filepath.Join(wantRoot, "Documents") {
		t.Fatalf("BuildChatDirectories() = %+v", got)
	}
}

func TestBuildChatDirectoriesSanitizesTraversalAndReservedNames(t *testing.T) {
	got, err := BuildChatDirectories("/tmp/downloads", "../CON/")
	if err != nil {
		t.Fatalf("BuildChatDirectories() error = %v", err)
	}
	if filepath.Dir(got.Root) != filepath.Clean("/tmp/downloads") {
		t.Fatalf("chat path escaped destination: %+v", got)
	}
	if filepath.Base(got.Root) == ".." || filepath.Base(got.Root) == "CON" {
		t.Fatalf("unsafe or reserved chat directory: %+v", got)
	}
}

func TestBuildChatDirectoriesRequiresDestination(t *testing.T) {
	if _, err := BuildChatDirectories("  ", "Channel"); err == nil {
		t.Fatal("expected destination validation error")
	}
}
