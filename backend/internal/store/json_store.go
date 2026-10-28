// Package store contains local persistence adapters. The JSON store is intended for the
// local-first desktop app and stores no Telegram credentials or message bodies beyond the
// job metadata needed to resume/review a download.
package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"

	"example.com/telegram-media-downloader/backend/internal/domain"
	"example.com/telegram-media-downloader/backend/internal/ports"
)

const stateFileVersion = 1

var ErrJobNotFound = ports.ErrJobNotFound

var _ ports.JobStore = (*JSONJobStore)(nil)

type JSONJobStore struct {
	mu   sync.RWMutex
	path string
	jobs map[string]domain.DownloadJob
}

type persistedJobs struct {
	Version int                           `json:"version"`
	Jobs    map[string]domain.DownloadJob `json:"jobs"`
}

func NewJSONJobStore(path string) (*JSONJobStore, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, errors.New("job store path is required")
	}
	absolutePath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolve job store path: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(absolutePath), 0o700); err != nil {
		return nil, fmt.Errorf("create job store directory: %w", err)
	}
	if err := recoverBackup(absolutePath); err != nil {
		return nil, fmt.Errorf("recover job store: %w", err)
	}

	store := &JSONJobStore{path: absolutePath, jobs: make(map[string]domain.DownloadJob)}
	data, err := os.ReadFile(absolutePath)
	if errors.Is(err, os.ErrNotExist) {
		return store, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read job store: %w", err)
	}
	var state persistedJobs
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("decode job store: %w", err)
	}
	if state.Version != stateFileVersion {
		return nil, fmt.Errorf("unsupported job store version %d (supported: %d)", state.Version, stateFileVersion)
	}
	if state.Jobs == nil {
		state.Jobs = make(map[string]domain.DownloadJob)
	}
	for id, job := range state.Jobs {
		if id != job.ID {
			return nil, fmt.Errorf("job store key %q does not match job ID %q", id, job.ID)
		}
		if err := job.Validate(); err != nil {
			return nil, fmt.Errorf("invalid persisted job %q: %w", id, err)
		}
		store.jobs[id] = cloneJob(job)
	}
	return store, nil
}

func (s *JSONJobStore) Save(ctx context.Context, job domain.DownloadJob) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := job.Validate(); err != nil {
		return fmt.Errorf("validate download job: %w", err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	old, existed := s.jobs[job.ID]
	s.jobs[job.ID] = cloneJob(job)
	if err := s.persistLocked(); err != nil {
		if existed {
			s.jobs[job.ID] = old
		} else {
			delete(s.jobs, job.ID)
		}
		return err
	}
	return nil
}

func (s *JSONJobStore) Get(ctx context.Context, jobID string) (domain.DownloadJob, error) {
	if err := ctx.Err(); err != nil {
		return domain.DownloadJob{}, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	job, ok := s.jobs[jobID]
	if !ok {
		return domain.DownloadJob{}, ErrJobNotFound
	}
	return cloneJob(job), nil
}

func (s *JSONJobStore) List(ctx context.Context) ([]domain.DownloadJob, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	ids := make([]string, 0, len(s.jobs))
	for id := range s.jobs {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	jobs := make([]domain.DownloadJob, 0, len(ids))
	for _, id := range ids {
		jobs = append(jobs, cloneJob(s.jobs[id]))
	}
	return jobs, nil
}

func (s *JSONJobStore) Delete(ctx context.Context, jobID string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	old, ok := s.jobs[jobID]
	if !ok {
		return ErrJobNotFound
	}
	delete(s.jobs, jobID)
	if err := s.persistLocked(); err != nil {
		s.jobs[jobID] = old
		return err
	}
	return nil
}

func (s *JSONJobStore) persistLocked() error {
	data, err := json.MarshalIndent(persistedJobs{Version: stateFileVersion, Jobs: s.jobs}, "", "  ")
	if err != nil {
		return fmt.Errorf("encode job store: %w", err)
	}
	dir := filepath.Dir(s.path)
	file, err := os.CreateTemp(dir, ".jobs-*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary job store: %w", err)
	}
	tempPath := file.Name()
	cleanTemp := true
	defer func() {
		if cleanTemp {
			_ = os.Remove(tempPath)
		}
	}()
	if err := file.Chmod(0o600); err != nil {
		_ = file.Close()
		return fmt.Errorf("secure temporary job store: %w", err)
	}
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		return fmt.Errorf("write temporary job store: %w", err)
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return fmt.Errorf("sync temporary job store: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close temporary job store: %w", err)
	}
	if err := replaceFile(tempPath, s.path); err != nil {
		return fmt.Errorf("replace job store: %w", err)
	}
	cleanTemp = false
	if dirFile, err := os.Open(dir); err == nil {
		_ = dirFile.Sync() // Directory sync is unsupported on some platforms; data file is already synced.
		_ = dirFile.Close()
	}
	return nil
}

func cloneJob(job domain.DownloadJob) domain.DownloadJob {
	job.Items = append([]domain.MediaItem(nil), job.Items...)
	job.Chat.Topics = append([]domain.Topic(nil), job.Chat.Topics...)
	job.FileTypes = append([]string(nil), job.FileTypes...)
	job.Files = append([]domain.FileTask(nil), job.Files...)
	for i := range job.Files {
		if job.Files[i].Progress != nil {
			p := *job.Files[i].Progress
			if p.Percent != nil {
				v := *p.Percent
				p.Percent = &v
			}
			if p.SpeedBytesPerSecond != nil {
				v := *p.SpeedBytesPerSecond
				p.SpeedBytesPerSecond = &v
			}
			if p.ETASeconds != nil {
				v := *p.ETASeconds
				p.ETASeconds = &v
			}
			job.Files[i].Progress = &p
		}
	}
	if job.Topic != nil {
		topic := *job.Topic
		job.Topic = &topic
	}
	if job.Selection != nil {
		selection := *job.Selection
		selection.Keys = append([]string(nil), selection.Keys...)
		selection.FileTypes = append([]string(nil), selection.FileTypes...)
		if selection.Topic != nil {
			topic := *selection.Topic
			selection.Topic = &topic
		}
		job.Selection = &selection
	}
	return job
}

func replaceFile(tempPath, targetPath string) error {
	if runtime.GOOS != "windows" {
		return os.Rename(tempPath, targetPath)
	}
	// os.Rename over an existing target is not consistently supported on Windows.
	// Keep a recoverable backup during replacement; NewJSONJobStore restores it if needed.
	backupPath := targetPath + ".bak"
	_ = os.Remove(backupPath)
	if _, err := os.Stat(targetPath); err == nil {
		if err := os.Rename(targetPath, backupPath); err != nil {
			return err
		}
	}
	if err := os.Rename(tempPath, targetPath); err != nil {
		if _, backupErr := os.Stat(backupPath); backupErr == nil {
			_ = os.Rename(backupPath, targetPath)
		}
		return err
	}
	_ = os.Remove(backupPath)
	return nil
}

func recoverBackup(targetPath string) error {
	backupPath := targetPath + ".bak"
	_, targetErr := os.Stat(targetPath)
	_, backupErr := os.Stat(backupPath)
	if errors.Is(targetErr, os.ErrNotExist) && backupErr == nil {
		return os.Rename(backupPath, targetPath)
	}
	if targetErr == nil && backupErr == nil {
		return os.Remove(backupPath)
	}
	if targetErr != nil && !errors.Is(targetErr, os.ErrNotExist) {
		return targetErr
	}
	if backupErr != nil && !errors.Is(backupErr, os.ErrNotExist) {
		return backupErr
	}
	return nil
}
