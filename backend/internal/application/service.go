// Package application contains use cases that coordinate the domain and infrastructure
// ports. It deliberately does not know about Flutter widgets or terminal rendering.
package application

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"example.com/telegram-media-downloader/backend/internal/safefs"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"example.com/telegram-media-downloader/backend/internal/domain"
	"example.com/telegram-media-downloader/backend/internal/ports"
)

type Telegram interface {
	ports.TelegramGateway
	ports.ExportProcessor
}

type RunRequest struct {
	JobID           string
	Chat            domain.Chat
	DestinationRoot string
	Filter          domain.FilterMode
	Takeout         bool
	GroupMedia      bool
	SkipSame        bool
	Continue        bool
}

type Event struct {
	Kind     string                      `json:"kind"`
	Job      *domain.DownloadJob         `json:"job,omitempty"`
	Progress *domain.ProgressEvent       `json:"progress,omitempty"`
	Summary  *domain.OrganizationSummary `json:"summary,omitempty"`
	Message  string                      `json:"message,omitempty"`
}

type RunResult struct {
	Job     domain.DownloadJob
	Summary domain.OrganizationSummary
}

var (
	// ErrPauseRequested is the cancellation cause used when the UI asks to pause.
	// A paused job is persisted and may be resumed with the same job ID.
	ErrPauseRequested = errors.New("pause requested")
	// ErrCancelRequested is distinct from a pause: cancelled jobs are terminal and
	// can only be retried as a new job.
	ErrCancelRequested = errors.New("cancel requested")
)

type Service struct {
	directoryMu      sync.Mutex
	chatRoots        map[string]string
	PrivateWorkRoot  string
	PreviewDirectory string
	previewMu        sync.Mutex
	previewCancel    context.CancelFunc
	previewWaiters   int
	Telegram         Telegram
	Jobs             ports.JobStore
	Organizer        ports.DownloadOrganizer
	Manifests        ports.ManifestStore

	mu          sync.Mutex
	activeChats map[int64]struct{}
	tdlSlot     chan struct{}
	libraryMu   sync.Mutex
	snapshots   map[string]mediaSnapshot
	chatCache   []domain.Chat
}

func NewService(telegram Telegram, jobs ports.JobStore, organizer ports.DownloadOrganizer, manifests ports.ManifestStore) *Service {
	return &Service{Telegram: telegram, Jobs: jobs, Organizer: organizer, Manifests: manifests, activeChats: make(map[int64]struct{}), tdlSlot: make(chan struct{}, 1)}
}

// Run executes the full lifecycle for one chat. It persists job-state changes before
// announcing them, filters the exported JSON to pending media, downloads to a per-job
// staging directory, then moves files to caption-based names and writes a manifest.
func (s *Service) Run(ctx context.Context, request RunRequest, emit func(Event)) (RunResult, error) {
	if err := s.validate(request); err != nil {
		return RunResult{}, err
	}
	if err := ctx.Err(); err != nil {
		return RunResult{}, err
	}

	existing, getErr := s.Jobs.Get(ctx, request.JobID)
	resuming := getErr == nil
	if getErr != nil && !errors.Is(getErr, ports.ErrJobNotFound) {
		return RunResult{}, fmt.Errorf("check existing download job: %w", getErr)
	}
	if resuming {
		if existing.State != domain.JobPaused {
			return RunResult{}, fmt.Errorf("job ID %q already exists in state %q; only paused jobs can resume", request.JobID, existing.State)
		}
		if existing.Chat.ID != request.Chat.ID {
			return RunResult{}, fmt.Errorf("paused job %q belongs to chat %d, not chat %d", request.JobID, existing.Chat.ID, request.Chat.ID)
		}
	}

	releaseChat, err := s.acquireChat(request.Chat.ID)
	if err != nil {
		return RunResult{}, err
	}
	defer releaseChat()

	filter := request.Filter
	if filter == "" {
		filter = domain.FilterVideosAndDocuments
	}
	if resuming {
		if filepath.Clean(request.DestinationRoot) != filepath.Clean(filepath.Dir(existing.Destination)) {
			return RunResult{}, fmt.Errorf("paused job %q must resume to its original destination", request.JobID)
		}
		if filter != existing.Filter {
			return RunResult{}, fmt.Errorf("paused job %q uses filter %q, not %q", request.JobID, existing.Filter, filter)
		}
	}
	var directories domain.ChatDirectories
	if resuming {
		directories.Root = existing.Destination
	} else {
		directories, err = s.resolveChatDirectories(ctx, request.DestinationRoot, request.Chat)
		if err != nil {
			return RunResult{}, err
		}
	}
	chatRoot := directories.Root
	if resuming {
		if err := s.ensurePrivateWork(ctx, &existing); err != nil {
			return RunResult{}, err
		}
	}
	jobWorkDir := existing.WorkDirectory
	if !resuming {
		jobWorkDir, err = s.newWorkDirectory()
		if err != nil {
			return RunResult{}, err
		}
	}
	stagingDir := filepath.Join(jobWorkDir, "staging")
	exportPath := filepath.Join(jobWorkDir, "export.json")
	pendingExportPath := filepath.Join(jobWorkDir, "pending-export.json")
	manifestPath := filepath.Join(chatRoot, ".telegram-media-manager-manifest.json")

	var job domain.DownloadJob
	if resuming {
		job = existing
		job.ErrorCode = ""
		job.ErrorMessage = ""
	} else {
		job = domain.DownloadJob{
			WorkDirectory: jobWorkDir, DestinationRoot: request.DestinationRoot, ID: request.JobID, Chat: request.Chat, State: domain.JobQueued,
			Destination: chatRoot, Filter: filter, Takeout: request.Takeout,
			GroupMedia: request.GroupMedia, SkipSame: request.SkipSame,
		}
		if err := s.Jobs.Save(ctx, job); err != nil {
			return RunResult{}, fmt.Errorf("persist queued download job: %w", err)
		}
		emitEvent(emit, Event{Kind: "job_state", Job: jobPointer(job)})
	}

	// tdl stores session/database state locally and cannot reliably run multiple
	// independent CLI instances concurrently. Jobs for different chats wait here,
	// remaining queued and cancellable instead of racing the local database.
	releaseTDL, err := s.acquireTDL(ctx)
	if err != nil {
		return s.fail(ctx, job, err, emit)
	}
	defer releaseTDL()
	if err := s.transition(ctx, &job, domain.JobPreparing, emit); err != nil {
		if ctx.Err() != nil {
			return s.fail(ctx, job, err, emit)
		}
		return RunResult{Job: job}, err
	}

	if err := safefs.Ensure(workBoundary(job), jobWorkDir); err != nil {
		return s.fail(ctx, job, fmt.Errorf("create job work directory: %w", err), emit)
	}
	if err := safefs.Ensure(workBoundary(job), stagingDir); err != nil {
		return s.fail(ctx, job, fmt.Errorf("create staging directory: %w", err), emit)
	}

	var pendingItems []domain.MediaItem
	var manifest *domain.Manifest
	summary := domain.OrganizationSummary{Outcomes: make([]domain.OrganizationOutcome, 0)}
	if resuming && len(job.Items) > 0 && regularFileExists(pendingExportPath) {
		// Keep the exact filtered export and item identity across pause/resume. tdl
		// uses that input to identify its previous download progress.
		pendingItems = append([]domain.MediaItem(nil), job.Items...)
		manifest, err = s.Manifests.Load(ctx, manifestPath, request.Chat.ID, request.Chat.Name)
		if err != nil {
			return s.fail(ctx, job, fmt.Errorf("load chat download manifest for resume: %w", err), emit)
		}
		var reconciled domain.OrganizationSummary
		var manifestChanged bool
		pendingItems, reconciled, manifestChanged, err = s.reconcilePendingItems(
			chatRoot, pendingItems, manifest, request.SkipSame || job.SkipSame,
		)
		if err != nil {
			return s.fail(ctx, job, fmt.Errorf("reconcile paused download: %w", err), emit)
		}
		mergeSummary(&summary, reconciled)
		if manifestChanged {
			if err := s.Manifests.Save(context.Background(), manifestPath, manifest); err != nil {
				return s.fail(context.Background(), job, fmt.Errorf("save reconciled chat manifest: %w", err), emit)
			}
		}
		job.Items = append([]domain.MediaItem(nil), pendingItems...)
		if err := s.Jobs.Save(context.Background(), job); err != nil {
			return s.fail(context.Background(), job, fmt.Errorf("persist reconciled media list: %w", err), emit)
		}
		if len(pendingItems) > 0 {
			exportData, err := os.ReadFile(pendingExportPath)
			if err != nil {
				return s.fail(ctx, job, fmt.Errorf("read pending export for resume: %w", err), emit)
			}
			filteredExport, err := s.Telegram.FilterExport(exportData, pendingItems)
			if err != nil {
				return s.fail(ctx, job, fmt.Errorf("refresh pending export for resume: %w", err), emit)
			}
			if err := os.WriteFile(pendingExportPath, filteredExport, 0o600); err != nil {
				return s.fail(ctx, job, fmt.Errorf("write refreshed pending export: %w", err), emit)
			}
		}
	} else {
		pendingItems, manifest, summary, err = s.preparePendingItems(
			ctx, request, filter, &job, exportPath, pendingExportPath, manifestPath, chatRoot,
		)
		if err != nil {
			return s.fail(ctx, job, fmt.Errorf("prepare media export: %w", err), emit)
		}
	}
	if len(pendingItems) == 0 {
		if err := s.transition(ctx, &job, domain.JobCompleted, emit); err != nil {
			return RunResult{Job: job, Summary: summary}, err
		}
		emitEvent(emit, Event{Kind: "job_completed", Job: jobPointer(job), Summary: summaryPointer(summary)})
		return RunResult{Job: job, Summary: summary}, nil
	}

	if err := s.transition(ctx, &job, domain.JobDownloading, emit); err != nil {
		if ctx.Err() != nil {
			return s.fail(ctx, job, err, emit)
		}
		return RunResult{Job: job, Summary: summary}, err
	}

	downloadRequest := domain.DownloadRequest{
		JobID: request.JobID, Chat: request.Chat, ExportPath: pendingExportPath, Destination: stagingDir,
		Filter: filter, Takeout: request.Takeout || job.Takeout, Continue: request.Continue || resuming,
		GroupMedia: request.GroupMedia || job.GroupMedia, SkipSame: request.SkipSame || job.SkipSame,
	}
	err = s.downloadWithRetry(ctx, downloadRequest, emit)
	if err != nil {
		return s.fail(ctx, job, fmt.Errorf("download media: %w", err), emit)
	}
	if err := ctx.Err(); err != nil {
		return s.fail(ctx, job, err, emit)
	}

	var organized domain.OrganizationSummary
	if transactionalOrganizer, ok := s.Organizer.(ports.ConfinedDownloadOrganizer); ok {
		organized, err = transactionalOrganizer.OrganizeDownloadsWithin(stagingDir, chatRoot, request.DestinationRoot, pendingItems, func(item domain.MediaItem, savedName string) error {
			manifest.MarkMoving(item, savedName)
			return s.Manifests.Save(context.Background(), manifestPath, manifest)
		})
	} else {
		organized, err = s.Organizer.OrganizeDownloads(stagingDir, chatRoot, pendingItems)
	}
	mergeSummary(&summary, organized)
	if err != nil {
		result, failErr := s.fail(ctx, job, fmt.Errorf("organize downloaded media: %w", err), func(event Event) {
			if event.Kind == "job_failed" || event.Kind == "job_paused" || event.Kind == "job_cancelled" {
				event.Summary = summaryPointer(summary)
			}
			emitEvent(emit, event)
		})
		result.Summary = summary
		return result, failErr
	}
	for _, outcome := range organized.Outcomes {
		if outcome.Status != domain.OrganizationRenamed {
			continue
		}
		for _, item := range pendingItems {
			if item.ChatID == outcome.ChatID && item.MessageID == outcome.MessageID && item.OriginalName == outcome.OriginalName {
				manifest.MarkCompleted(item, filepath.Base(outcome.SavedPath))
				break
			}
		}
	}
	if organized.Renamed > 0 {
		// Files are already finalized at this point. Persist the manifest even if the
		// caller closed the window just after the transfer completed, otherwise reruns
		// could re-download and duplicate successfully saved files.
		if err := s.Manifests.Save(context.Background(), manifestPath, manifest); err != nil {
			return s.fail(context.Background(), job, fmt.Errorf("save chat download manifest: %w", err), emit)
		}
	}
	if organized.Failed > 0 {
		message := fmt.Sprintf("download finished with %d file organization failure(s)", organized.Failed)
		job.ErrorCode = string(domain.ErrorCodeDownloadFailed)
		job.ErrorMessage = message
		if err := s.transition(context.Background(), &job, domain.JobFailed, emit); err != nil {
			return RunResult{Job: job, Summary: summary}, err
		}
		emitEvent(emit, Event{Kind: "job_failed", Job: jobPointer(job), Summary: summaryPointer(summary), Message: message})
		return RunResult{Job: job, Summary: summary}, errors.New(message)
	}
	if err := s.transition(context.Background(), &job, domain.JobCompleted, emit); err != nil {
		return RunResult{Job: job, Summary: summary}, err
	}
	emitEvent(emit, Event{Kind: "job_completed", Job: jobPointer(job), Summary: summaryPointer(summary)})
	s.cleanupSuccess(jobWorkDir, stagingDir, exportPath, pendingExportPath)
	return RunResult{Job: job, Summary: summary}, nil
}

// Resume loads a persisted paused job and continues with the exact same job identity,
// export file, staging directory and tdl resume flags.
func (s *Service) Resume(ctx context.Context, jobID string, emit func(Event)) (RunResult, error) {
	job, err := s.Jobs.Get(ctx, jobID)
	if err != nil {
		return RunResult{}, fmt.Errorf("load job for resume: %w", err)
	}
	if job.State != domain.JobPaused {
		return RunResult{}, fmt.Errorf("job %q is %q; only paused jobs can resume", jobID, job.State)
	}
	if job.Library {
		return s.continueMedia(ctx, job, false, emit)
	}
	return s.Run(ctx, RunRequest{
		JobID: job.ID, Chat: job.Chat, DestinationRoot: filepath.Dir(job.Destination), Filter: job.Filter,
		Takeout: job.Takeout, GroupMedia: job.GroupMedia, SkipSame: job.SkipSame, Continue: true,
	}, emit)
}

// CancelPersistedJob cancels a paused/queued job when no child process is running.
// Active jobs are cancelled via their context in the app process controller instead.
func (s *Service) CancelPersistedJob(ctx context.Context, jobID string, emit func(Event)) (domain.DownloadJob, error) {
	job, err := s.Jobs.Get(ctx, jobID)
	if err != nil {
		return domain.DownloadJob{}, fmt.Errorf("load job for cancellation: %w", err)
	}
	if job.State != domain.JobPaused && job.State != domain.JobQueued {
		return domain.DownloadJob{}, fmt.Errorf("job %q is %q; only inactive queued/paused jobs can be cancelled directly", jobID, job.State)
	}
	if job.Library {
		for i := range job.Files {
			if job.Files[i].State != "completed" && job.Files[i].State != "failed" {
				job.Files[i].State = "cancelled"
			}
		}
	}
	return s.cancelJob(ctx, &job, emit)
}

func (s *Service) preparePendingItems(
	ctx context.Context,
	request RunRequest,
	filter domain.FilterMode,
	job *domain.DownloadJob,
	exportPath, pendingExportPath, manifestPath, chatRoot string,
) ([]domain.MediaItem, *domain.Manifest, domain.OrganizationSummary, error) {
	if err := s.Telegram.ExportChat(ctx, request.Chat, exportPath); err != nil {
		return nil, nil, domain.OrganizationSummary{}, fmt.Errorf("export chat messages: %w", err)
	}
	exportData, err := os.ReadFile(exportPath)
	if err != nil {
		return nil, nil, domain.OrganizationSummary{}, fmt.Errorf("read chat export: %w", err)
	}
	allItems, err := s.Telegram.ParseExport(exportData)
	if err != nil {
		return nil, nil, domain.OrganizationSummary{}, fmt.Errorf("parse chat export: %w", err)
	}
	includedItems, err := filterItems(allItems, filter)
	if err != nil {
		return nil, nil, domain.OrganizationSummary{}, err
	}
	manifest, err := s.Manifests.Load(ctx, manifestPath, request.Chat.ID, request.Chat.Name)
	if err != nil {
		return nil, nil, domain.OrganizationSummary{}, fmt.Errorf("load chat download manifest: %w", err)
	}
	pendingItems, summary, manifestChanged, err := s.reconcilePendingItems(chatRoot, includedItems, manifest, request.SkipSame)
	if err != nil {
		return nil, nil, domain.OrganizationSummary{}, err
	}
	if manifestChanged {
		if err := s.Manifests.Save(ctx, manifestPath, manifest); err != nil {
			return nil, nil, domain.OrganizationSummary{}, fmt.Errorf("update chat manifest for missing/in-progress media: %w", err)
		}
	}
	job.Items = append([]domain.MediaItem(nil), pendingItems...)
	if err := s.Jobs.Save(ctx, *job); err != nil {
		return nil, nil, domain.OrganizationSummary{}, fmt.Errorf("persist exported media list: %w", err)
	}
	if len(pendingItems) > 0 {
		pendingExport, err := s.Telegram.FilterExport(exportData, pendingItems)
		if err != nil {
			return nil, nil, domain.OrganizationSummary{}, fmt.Errorf("filter export to pending media: %w", err)
		}
		if err := os.WriteFile(pendingExportPath, pendingExport, 0o600); err != nil {
			return nil, nil, domain.OrganizationSummary{}, fmt.Errorf("write pending-media export: %w", err)
		}
	}
	return pendingItems, manifest, summary, nil
}

func (s *Service) reconcilePendingItems(
	chatRoot string,
	items []domain.MediaItem,
	manifest *domain.Manifest,
	skipCompleted bool,
) ([]domain.MediaItem, domain.OrganizationSummary, bool, error) {
	pending := make([]domain.MediaItem, 0, len(items))
	summary := domain.OrganizationSummary{Outcomes: make([]domain.OrganizationOutcome, 0)}
	changed := false
	for _, item := range items {
		entry, exists := manifest.Items[item.Key()]
		if !exists {
			pending = append(pending, item)
			continue
		}
		if entry.Status == "moving" {
			present, err := completedFilePresent(chatRoot, item, entry)
			if err != nil {
				return nil, summary, changed, fmt.Errorf("verify in-progress media %q: %w", entry.SavedName, err)
			}
			if present {
				entry.Status = "completed"
				manifest.Items[item.Key()] = entry
				changed = true
				summary.AlreadyCompleted++
				summary.Outcomes = append(summary.Outcomes, domain.OrganizationOutcome{
					ChatID: item.ChatID, MessageID: item.MessageID, OriginalName: item.OriginalName,
					SavedPath: filepath.Join(chatRoot, categoryFolder(domain.ClassifyMedia(item.OriginalName)), entry.SavedName),
					Status:    domain.OrganizationAlreadyCompleted, Reason: "recovered a file moved before the previous run saved completion state",
				})
				continue
			}
			delete(manifest.Items, item.Key())
			changed = true
			pending = append(pending, item)
			continue
		}
		if entry.Status == "completed" {
			present, err := completedFilePresent(chatRoot, item, entry)
			if err != nil {
				return nil, summary, changed, fmt.Errorf("verify completed media %q: %w", entry.SavedName, err)
			}
			if present && skipCompleted {
				summary.AlreadyCompleted++
				summary.Outcomes = append(summary.Outcomes, domain.OrganizationOutcome{
					ChatID: item.ChatID, MessageID: item.MessageID, OriginalName: item.OriginalName,
					SavedPath: filepath.Join(chatRoot, categoryFolder(domain.ClassifyMedia(item.OriginalName)), entry.SavedName),
					Status:    domain.OrganizationAlreadyCompleted, Reason: "file exists and is recorded as completed in the chat manifest",
				})
				continue
			}
			if !present {
				delete(manifest.Items, item.Key())
				changed = true
			}
		}
		pending = append(pending, item)
	}
	return pending, summary, changed, nil
}

func regularFileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}

const maxDownloadAttempts = 3

// downloadWithRetry retries only errors explicitly classified as transient by the tdl
// adapter. Rate limits are not retried automatically because the server's required wait
// must be honored; authorization and permanent storage errors are surfaced immediately.
func (s *Service) downloadWithRetry(ctx context.Context, request domain.DownloadRequest, emit func(Event)) error {
	var lastErr error
	for attempt := 1; attempt <= maxDownloadAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		attemptRequest := request
		if attempt > 1 {
			attemptRequest.Continue = true
		}
		err := s.Telegram.Download(ctx, attemptRequest, func(progress domain.ProgressEvent) {
			if progress.JobID == "" {
				progress.JobID = request.JobID
			}
			progressCopy := progress
			emitEvent(emit, Event{Kind: "download_progress", Progress: &progressCopy})
		})
		if err == nil {
			return nil
		}
		lastErr = err
		if attempt == maxDownloadAttempts || !isRetryableDownloadError(err) || ctx.Err() != nil {
			return err
		}
		delay := time.Duration(250*(1<<(attempt-1))) * time.Millisecond
		emitEvent(emit, Event{Kind: "download_retry", Message: fmt.Sprintf("Temporary download failure; retrying attempt %d of %d in %s.", attempt+1, maxDownloadAttempts, delay)})
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
	return lastErr
}

func isRetryableDownloadError(err error) bool {
	var appErr *domain.AppError
	if !errors.As(err, &appErr) || !appErr.Retryable {
		return false
	}
	// Don't retry Telegram flood-waits without an explicit, parsed retry-after value.
	return appErr.Code != domain.ErrorCodeRateLimited && appErr.Code != domain.ErrorCodeUnauthorized
}

func (s *Service) ListJobs(ctx context.Context) ([]domain.DownloadJob, error) {
	jobs, err := s.Jobs.List(ctx)
	if err != nil {
		return nil, err
	}
	for j := range jobs {
		// Availability is a view of the current filesystem, never a history mutation.
		jobs[j].Files = append([]domain.FileTask(nil), jobs[j].Files...)
		for f := range jobs[j].Files {
			file := &jobs[j].Files[f]
			if file.State != "completed" {
				continue
			}
			file.Availability = "unknown"
			if file.SavedPath == "" {
				continue
			}
			info, statErr := os.Stat(file.SavedPath)
			if statErr == nil && info.Mode().IsRegular() {
				file.Availability = "available"
				continue
			}
			file.Availability = "unavailable"
			root := jobs[j].DestinationRoot
			if root == "" {
				root = jobs[j].Destination
			}
			if rootInfo, rootErr := os.Stat(root); rootErr == nil && rootInfo.IsDir() && os.IsNotExist(statErr) {
				file.Availability = "missing"
			}
		}
	}
	return jobs, nil
}

// RecoverInterruptedJobs normalizes jobs left in an active state when the app or
// machine stopped unexpectedly. Active transfers become paused so the user can
// explicitly resume them; jobs already in the cancelling transition become cancelled.
func (s *Service) RecoverInterruptedJobs(ctx context.Context) (int, error) {
	if s == nil || s.Jobs == nil {
		return 0, errors.New("job store is not configured")
	}
	jobs, err := s.Jobs.List(ctx)
	if err != nil {
		return 0, fmt.Errorf("list jobs for recovery: %w", err)
	}
	recovered := 0
	for _, job := range jobs {
		changed := false
		switch job.State {
		case domain.JobQueued, domain.JobPreparing, domain.JobDownloading:
			next, transitionErr := job.Transition(domain.JobPausing)
			if transitionErr != nil {
				return recovered, fmt.Errorf("begin recovery for job %q: %w", job.ID, transitionErr)
			}
			job = next
			changed = true
			fallthrough
		case domain.JobPausing:
			next, transitionErr := job.Transition(domain.JobPaused)
			if transitionErr != nil {
				return recovered, fmt.Errorf("finish recovery for job %q: %w", job.ID, transitionErr)
			}
			job = next
			job.ErrorCode = ""
			job.ErrorMessage = ""
			changed = true
		case domain.JobCancelling:
			next, transitionErr := job.Transition(domain.JobCancelled)
			if transitionErr != nil {
				return recovered, fmt.Errorf("recover cancellation for job %q: %w", job.ID, transitionErr)
			}
			job = next
			job.ErrorCode = string(domain.ErrorCodeInterrupted)
			job.ErrorMessage = "Cancellation completed during startup recovery."
			changed = true
		}
		if changed {
			if job.Library {
				for i := range job.Files {
					if job.Files[i].State != "completed" && job.Files[i].State != "failed" {
						if job.State == domain.JobPaused {
							job.Files[i].State = "paused"
						} else {
							job.Files[i].State = "cancelled"
						}
					}
				}
			}
			if err := s.Jobs.Save(ctx, job); err != nil {
				return recovered, fmt.Errorf("persist recovered job %q: %w", job.ID, err)
			}
			recovered++
		}
	}
	return recovered, nil
}

func (s *Service) validate(request RunRequest) error {
	if s == nil || s.Telegram == nil || s.Jobs == nil || s.Organizer == nil || s.Manifests == nil {
		return errors.New("download service is not fully configured")
	}
	if strings.TrimSpace(request.JobID) == "" {
		return errors.New("job ID is required")
	}
	if request.Chat.ID == 0 {
		return errors.New("selected chat must have a non-zero ID")
	}
	if strings.TrimSpace(request.DestinationRoot) == "" {
		return errors.New("destination root is required")
	}
	if !filepath.IsAbs(request.DestinationRoot) {
		return errors.New("destination root must be an absolute path")
	}
	info, err := os.Stat(request.DestinationRoot)
	if err != nil {
		return fmt.Errorf("destination directory is not available; check that the selected drive is mounted: %w", err)
	}
	if !info.IsDir() {
		return errors.New("destination root must be an existing directory")
	}
	if request.Filter != "" && !request.Filter.Valid() {
		return fmt.Errorf("unsupported media filter %q", request.Filter)
	}
	return nil
}

func (s *Service) cliSlot() chan struct{} {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.tdlSlot == nil {
		s.tdlSlot = make(chan struct{}, 1)
	}
	return s.tdlSlot
}
func (s *Service) acquireTDL(ctx context.Context) (func(), error) {
	slot := s.cliSlot()
	// Register priority intent and cancel the current preview under the same lock
	// used to claim preview ownership. A new preview cannot slip between them.
	s.previewMu.Lock()
	s.previewWaiters++
	if s.previewCancel != nil {
		s.previewCancel()
	}
	s.previewMu.Unlock()
	defer func() { s.previewMu.Lock(); s.previewWaiters--; s.previewMu.Unlock() }()
	select {
	case slot <- struct{}{}:
		return func() { <-slot }, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (s *Service) acquireChat(chatID int64) (func(), error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.activeChats == nil {
		s.activeChats = make(map[int64]struct{})
	}
	if _, exists := s.activeChats[chatID]; exists {
		return nil, fmt.Errorf("a download for chat %d is already active", chatID)
	}
	s.activeChats[chatID] = struct{}{}
	return func() {
		s.mu.Lock()
		delete(s.activeChats, chatID)
		s.mu.Unlock()
	}, nil
}

func (s *Service) transition(ctx context.Context, job *domain.DownloadJob, to domain.JobState, emit func(Event)) error {
	next, err := job.Transition(to)
	if err != nil {
		return err
	}
	if err := s.Jobs.Save(ctx, next); err != nil {
		return fmt.Errorf("persist job state %s: %w", to, err)
	}
	*job = next
	emitEvent(emit, Event{Kind: "job_state", Job: jobPointer(*job)})
	return nil
}

func (s *Service) fail(ctx context.Context, job domain.DownloadJob, cause error, emit func(Event)) (RunResult, error) {
	if ctx.Err() != nil {
		if errors.Is(context.Cause(ctx), ErrPauseRequested) {
			paused, pauseErr := s.pauseJob(context.Background(), &job, emit)
			if pauseErr != nil {
				return RunResult{Job: paused}, errors.Join(ctx.Err(), pauseErr)
			}
			return RunResult{Job: paused}, nil
		}
		cancelled, cancelErr := s.cancelJob(context.Background(), &job, emit)
		if cancelErr != nil {
			return RunResult{Job: cancelled}, errors.Join(ctx.Err(), cancelErr)
		}
		return RunResult{Job: cancelled}, ctx.Err()
	}
	job.ErrorCode = string(errorCodeFor(cause))
	job.ErrorMessage = truncateError(cause.Error(), 500)
	if domain.CanTransition(job.State, domain.JobFailed) {
		if err := s.transition(context.Background(), &job, domain.JobFailed, emit); err != nil {
			return RunResult{Job: job}, errors.Join(cause, err)
		}
	} else {
		return RunResult{Job: job}, cause
	}
	emitEvent(emit, Event{Kind: "job_failed", Job: jobPointer(job), Message: job.ErrorMessage})
	return RunResult{Job: job}, cause
}

func (s *Service) pauseJob(ctx context.Context, job *domain.DownloadJob, emit func(Event)) (domain.DownloadJob, error) {
	if job.State == domain.JobPaused {
		return *job, nil
	}
	if job.State == domain.JobQueued || job.State == domain.JobPreparing || job.State == domain.JobDownloading {
		if err := s.transition(ctx, job, domain.JobPausing, emit); err != nil {
			return *job, err
		}
	}
	if job.State == domain.JobPausing {
		if err := s.transition(ctx, job, domain.JobPaused, emit); err != nil {
			return *job, err
		}
	}
	if job.State != domain.JobPaused {
		return *job, fmt.Errorf("cannot pause job in state %q", job.State)
	}
	job.ErrorCode = ""
	job.ErrorMessage = ""
	if err := s.Jobs.Save(ctx, *job); err != nil {
		return *job, fmt.Errorf("persist paused job: %w", err)
	}
	emitEvent(emit, Event{Kind: "job_paused", Job: jobPointer(*job), Message: "Download paused; staging files and resume data were preserved."})
	return *job, nil
}

func (s *Service) cancelJob(ctx context.Context, job *domain.DownloadJob, emit func(Event)) (domain.DownloadJob, error) {
	switch job.State {
	case domain.JobQueued, domain.JobPaused:
		if err := s.transition(ctx, job, domain.JobCancelled, emit); err != nil {
			return *job, err
		}
	case domain.JobPreparing, domain.JobDownloading, domain.JobPausing:
		if err := s.transition(ctx, job, domain.JobCancelling, emit); err != nil {
			return *job, err
		}
		if err := s.transition(ctx, job, domain.JobCancelled, emit); err != nil {
			return *job, err
		}
	case domain.JobCancelling:
		if err := s.transition(ctx, job, domain.JobCancelled, emit); err != nil {
			return *job, err
		}
	case domain.JobCancelled:
		return *job, nil
	default:
		return *job, fmt.Errorf("cannot cancel job in state %q", job.State)
	}
	job.ErrorCode = string(domain.ErrorCodeInterrupted)
	job.ErrorMessage = "download cancelled; completed files were preserved and staging files were retained"
	if err := s.Jobs.Save(ctx, *job); err != nil {
		return *job, fmt.Errorf("persist cancelled job: %w", err)
	}
	emitEvent(emit, Event{Kind: "job_cancelled", Job: jobPointer(*job), Message: job.ErrorMessage})
	return *job, nil
}

func filterItems(items []domain.MediaItem, filter domain.FilterMode) ([]domain.MediaItem, error) {
	result := make([]domain.MediaItem, 0, len(items))
	for _, item := range items {
		include, err := domain.ShouldIncludeMedia(item.OriginalName, filter)
		if err != nil {
			return nil, err
		}
		if include {
			result = append(result, item)
		}
	}
	return result, nil
}

func workDirectoryName(jobID string) string {
	hash := sha256.Sum256([]byte(jobID))
	return "job-" + hex.EncodeToString(hash[:8])
}

func mergeSummary(target *domain.OrganizationSummary, source domain.OrganizationSummary) {
	target.Renamed += source.Renamed
	target.Skipped += source.Skipped
	target.Failed += source.Failed
	target.AlreadyCompleted += source.AlreadyCompleted
	target.Outcomes = append(target.Outcomes, source.Outcomes...)
}

func emitEvent(emit func(Event), event Event) {
	if emit != nil {
		emit(event)
	}
}

func jobPointer(job domain.DownloadJob) *domain.DownloadJob                         { return &job }
func summaryPointer(summary domain.OrganizationSummary) *domain.OrganizationSummary { return &summary }

func errorCodeFor(err error) domain.ErrorCode {
	var appErr *domain.AppError
	if errors.As(err, &appErr) && appErr.Code != "" {
		return appErr.Code
	}
	return domain.ErrorCodeDownloadFailed
}

func truncateError(message string, max int) string {
	message = strings.TrimSpace(message)
	if len(message) <= max {
		return message
	}
	return message[:max] + "…"
}

func completedFilePresent(chatRoot string, item domain.MediaItem, entry domain.ManifestEntry) (bool, error) {
	if strings.TrimSpace(entry.SavedName) == "" {
		return false, nil
	}
	if item.ContentID != "" && entry.ContentID != "" && item.ContentID != entry.ContentID {
		return false, nil
	}
	if (entry.ChatID != 0 && entry.ChatID != item.ChatID) || (entry.MessageID != 0 && entry.MessageID != item.MessageID) || (entry.OriginalName != "" && entry.OriginalName != item.OriginalName) {
		return false, nil
	}
	relative, err := savedRelative(item, entry)
	if err != nil {
		return false, err
	}
	boundary, name, err := safefs.Open(chatRoot, filepath.Join(chatRoot, relative))
	if err != nil {
		return false, err
	}
	defer boundary.Close()
	info, err := boundary.Stat(name)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	expected := item.SizeBytes
	if expected <= 0 {
		expected = entry.SizeBytes
	}
	return info.Mode().IsRegular() && (expected <= 0 || info.Size() == expected), nil
}

func categoryFolder(category domain.MediaCategory) string {
	switch category {
	case domain.CategoryVideo:
		return "Videos"
	case domain.CategoryDocument:
		return "Documents"
	default:
		return "Attachments"
	}
}

func (s *Service) cleanupSuccess(jobWorkDir, stagingDir, exportPath, pendingExportPath string) {
	_ = os.Remove(exportPath)
	_ = os.Remove(pendingExportPath)
	entries, err := os.ReadDir(stagingDir)
	if err != nil || len(entries) != 0 {
		return
	}
	_ = os.Remove(stagingDir)
	_ = os.Remove(jobWorkDir)
	_ = os.Remove(filepath.Dir(jobWorkDir))
}
