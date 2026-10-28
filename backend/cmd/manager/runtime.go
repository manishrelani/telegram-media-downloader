package main

import (
	"path/filepath"
	"runtime"
)

func runtimeBinary(configured, executable string) string {
	if configured != "" {
		return configured
	}
	name := "tdl-tmm"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	return filepath.Join(filepath.Dir(executable), name)
}
