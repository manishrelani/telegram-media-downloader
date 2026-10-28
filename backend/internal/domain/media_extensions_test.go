package domain

import (
	"reflect"
	"testing"
)

func TestExtensionsForFilterReturnsStableAllowlist(t *testing.T) {
	videos, err := ExtensionsForFilter(FilterVideosOnly)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(videos, []string{"3gp", "avi", "flv", "m2ts", "m2v", "m4v", "mkv", "mov", "mp4", "mpeg", "mpg", "mts", "ogv", "ts", "webm", "wmv"}) {
		t.Fatalf("video extensions = %#v", videos)
	}
	all, err := ExtensionsForFilter(FilterAllAttachments)
	if err != nil {
		t.Fatal(err)
	}
	if all != nil {
		t.Fatalf("all-attachments extensions = %#v, want nil allowlist", all)
	}
}

func TestExtensionsForFilterRejectsUnknownMode(t *testing.T) {
	if _, err := ExtensionsForFilter(FilterMode("nope")); err == nil {
		t.Fatal("expected invalid filter mode error")
	}
}
