package main

import (
	"path/filepath"
	"runtime"
	"testing"
)

func TestRuntimeUsesSiblingOrExplicitOverride(t *testing.T) {
	name := "tdl-tmm"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	executable := filepath.Join(t.TempDir(), "telegram-media-downloader-backend")
	if got := runtimeBinary("", executable); got != filepath.Join(filepath.Dir(executable), name) {
		t.Fatalf("unsafe default runtime %q", got)
	}
	override := filepath.Join(t.TempDir(), "trusted-runtime")
	if got := runtimeBinary(override, executable); got != override {
		t.Fatalf("override ignored: %s", got)
	}
}
