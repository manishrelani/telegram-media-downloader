package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"example.com/telegram-media-downloader/backend/internal/app"
	"example.com/telegram-media-downloader/backend/internal/application"
	"example.com/telegram-media-downloader/backend/internal/filesystem"
	"example.com/telegram-media-downloader/backend/internal/protocol"
	"example.com/telegram-media-downloader/backend/internal/store"
	"example.com/telegram-media-downloader/backend/internal/tdl"
)

func main() {
	executable, err := os.Executable()
	if err != nil {
		fmt.Fprintln(os.Stderr, "backend startup failed: resolve executable:", err)
		os.Exit(1)
	}
	binary := runtimeBinary(os.Getenv("TDL_PATH"), executable)
	adapter := tdl.DomainAdapter{Client: tdl.Client{Exec: tdl.OSExecutor{Binary: binary}}}
	configDir, err := os.UserConfigDir()
	if err != nil {
		fmt.Fprintln(os.Stderr, "backend startup failed: resolve user config directory:", err)
		os.Exit(1)
	}
	stateDir := os.Getenv("TMM_STATE_DIR")
	if stateDir == "" {
		stateDir = filepath.Join(configDir, "telegram-media-manager")
	}
	jobStore, err := store.NewJSONJobStore(filepath.Join(stateDir, "jobs.json"))
	if err != nil {
		fmt.Fprintln(os.Stderr, "backend startup failed: open job store:", err)
		os.Exit(1)
	}
	downloads := application.NewService(adapter, jobStore, filesystem.DownloadOrganizer{}, filesystem.JSONManifestStore{})
	downloads.PrivateWorkRoot = filepath.Join(stateDir, "work")
	downloads.PreviewDirectory = filepath.Join(stateDir, "preview-cache-v2")
	if recovered, recoveryErr := downloads.RecoverInterruptedJobs(context.Background()); recoveryErr != nil {
		fmt.Fprintln(os.Stderr, "backend startup warning: recover interrupted jobs:", recoveryErr)
	} else if recovered > 0 {
		fmt.Fprintf(os.Stderr, "backend startup: recovered %d interrupted job(s) as paused/cancelled\n", recovered)
	}
	handler := app.NewWithDownloadService(adapter, downloads)
	ctx, cancel := context.WithCancel(context.Background())
	err = protocol.Serve(ctx, os.Stdin, os.Stdout, handler)
	// Let the handler interrupt its own active tdl children and persist resumable
	// state before cancelling the parent context. Reversing this order would turn a
	// graceful shutdown into an indistinguishable hard cancellation.
	handler.Close()
	cancel()
	if err != nil && !errors.Is(err, context.Canceled) {
		// Keep stdout reserved for JSONL protocol messages. Startup/runtime diagnostics go to stderr.
		fmt.Fprintln(os.Stderr, "backend stopped:", err)
		os.Exit(1)
	}
}
