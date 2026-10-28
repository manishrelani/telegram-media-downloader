package application

import (
	"context"
	"errors"
	"example.com/telegram-media-downloader/backend/internal/domain"
	"example.com/telegram-media-downloader/backend/internal/safefs"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

// New external-process work happens in app-owned private storage, never inside a
// shared save folder. The path is persisted so restart/retry keeps its input.
func (s *Service) newWorkDirectory() (string, error) {
	if s.PrivateWorkRoot != "" {
		if err := os.MkdirAll(s.PrivateWorkRoot, 0700); err != nil {
			return "", err
		}
		info, err := os.Lstat(s.PrivateWorkRoot)
		if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("private work directory is not a directory")
		}
	}
	return os.MkdirTemp(s.PrivateWorkRoot, "tmm-download-")
}
func workBoundary(job domain.DownloadJob) string {
	if job.WorkDirectory != "" {
		return job.WorkDirectory
	}
	if job.DestinationRoot != "" {
		return job.DestinationRoot
	}
	return filepath.Dir(job.Destination)
}

// Migrate old recovery data without following links in a shared destination.
// Persist only after the copy succeeds; keep the old data for recovery on failure.
func (s *Service) ensurePrivateWork(ctx context.Context, job *domain.DownloadJob) error {
	if job.WorkDirectory != "" {
		return nil
	}
	previous := mediaWorkDir(*job)
	anchor := job.DestinationRoot
	if anchor == "" {
		anchor = filepath.Dir(job.Destination)
	}
	source, rel, err := safefs.Open(anchor, previous)
	if err != nil {
		return fmt.Errorf("validate recovery directory: %w", err)
	}
	defer source.Close()
	private, err := s.newWorkDirectory()
	if err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			_ = os.RemoveAll(private)
		}
	}()
	destination, err := os.OpenRoot(private)
	if err != nil {
		return err
	}
	defer destination.Close()
	oldRoot, err := source.OpenRoot(rel)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	if oldRoot != nil {
		defer oldRoot.Close()
		err = fs.WalkDir(oldRoot.FS(), ".", func(name string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if entry.Type()&os.ModeSymlink != 0 {
				return fmt.Errorf("linked recovery data is unsafe: %s", name)
			}
			if entry.IsDir() {
				return destination.MkdirAll(name, 0700)
			}
			info, err := entry.Info()
			if err != nil {
				return err
			}
			if !info.Mode().IsRegular() {
				return fmt.Errorf("nonregular recovery data: %s", name)
			}
			input, err := oldRoot.Open(name)
			if err != nil {
				return err
			}
			defer input.Close()
			output, err := destination.OpenFile(name, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
			if err != nil {
				return err
			}
			_, copyErr := io.Copy(output, input)
			closeErr := output.Close()
			return errors.Join(copyErr, closeErr)
		})
		if err != nil {
			return fmt.Errorf("migrate recovery data: %w", err)
		}
	}
	next := *job
	next.WorkDirectory = private
	if err := s.Jobs.Save(ctx, next); err != nil {
		return err
	}
	*job = next
	committed = true
	return nil
}
