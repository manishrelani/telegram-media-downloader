package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"example.com/telegram-media-downloader/backend/internal/application"
	"example.com/telegram-media-downloader/backend/internal/domain"
	"example.com/telegram-media-downloader/backend/internal/ports"
	"example.com/telegram-media-downloader/backend/internal/protocol"
)

type Telegram interface {
	ports.TelegramGateway
	ports.ManagedDownloadCapabilities
}

type Handler struct {
	Telegram   Telegram
	Downloads  *application.Service
	mu         sync.Mutex
	jobs       map[string]context.CancelCauseFunc
	wg         sync.WaitGroup
	closed     bool
	tdlVersion string
}

func New(t Telegram) *Handler {
	return NewWithDownloadService(t, nil)
}

func NewWithDownloadService(t Telegram, downloads *application.Service) *Handler {
	return &Handler{Telegram: t, Downloads: downloads, jobs: make(map[string]context.CancelCauseFunc)}
}

func (h *Handler) Handle(ctx context.Context, req protocol.Request, emit func(protocol.Message)) {
	switch req.Action {
	case "request_thumbnails", "list_media", "start_media_download", "retry_download", "open_download_file", "cancel_media_request":
		h.handleMediaRequest(ctx, req, emit)
	case "health":
		// Health is also the explicit refresh boundary: re-read the executable version
		// so an upgraded tdl binary is recognized without requiring a backend restart.
		version, err := h.refreshTDLVersion(ctx)
		if err != nil {
			emitError(emit, req.ID, "tdl_unavailable", err)
			return
		}
		emit(protocol.Message{ID: req.ID, Event: "result", Data: map[string]any{
			"ready": true, "tdlVersion": version,
			"minimumSafeManagedDownloadVersion": h.Telegram.MinimumSafeManagedDownloadVersion(),
			"managedDownloadsSupported":         h.Telegram.ManagedDownloadsSupported(version),
			"supportsGracefulPause":             h.Telegram.SafeInterruptionSupported(version),
		}})
	case "list_chats":
		var chats []domain.Chat
		var err error
		if h.Downloads != nil {
			chats, err = h.Downloads.ListChats(ctx)
		} else {
			chats, err = h.Telegram.ListChats(ctx)
		}
		if err != nil {
			emitError(emit, req.ID, "chat_list_failed", err)
			return
		}
		emit(protocol.Message{ID: req.ID, Event: "result", Data: map[string]any{"chats": chats}})
	case "list_jobs":
		if h.Downloads == nil {
			emitError(emit, req.ID, "unavailable", fmt.Errorf("download service is not configured"))
			return
		}
		jobs, err := h.Downloads.ListJobs(ctx)
		if err != nil {
			emitError(emit, req.ID, "list_jobs_failed", err)
			return
		}
		emit(protocol.Message{ID: req.ID, Event: "result", Data: map[string]any{"jobs": jobs}})
	case "start_chat_download":
		if h.Downloads == nil {
			emitError(emit, req.ID, "unavailable", fmt.Errorf("download service is not configured"))
			return
		}
		var p protocol.StartChatDownloadParams
		if err := decodeParams(req.Params, &p); err != nil {
			emitError(emit, req.ID, "invalid_params", err)
			return
		}
		if p.Chat.ID == 0 || strings.TrimSpace(p.Chat.Name) == "" {
			emitError(emit, req.ID, "invalid_params", fmt.Errorf("chat must include a non-zero ID and display name"))
			return
		}
		if !h.requireManagedDownloadVersion(ctx, emit, req.ID) {
			return
		}
		jobCtx, cancel, beginErr := h.beginJob(ctx, req.ID)
		if beginErr != nil {
			emitError(emit, req.ID, "duplicate_job_id", beginErr)
			return
		}
		emit(protocol.Message{ID: req.ID, Event: "job_accepted", Data: map[string]any{"jobId": req.ID}})
		go func() {
			defer h.finishJob(req.ID, cancel)
			terminalEmitted := false
			request := application.RunRequest{
				JobID:           req.ID,
				Chat:            domain.Chat{ID: p.Chat.ID, Type: p.Chat.Type, Name: p.Chat.Name, Username: p.Chat.Username},
				DestinationRoot: p.DestinationRoot, Filter: domain.FilterMode(p.Filter),
				Takeout: p.Takeout, GroupMedia: p.Group, SkipSame: p.SkipSame,
			}
			_, err := h.Downloads.Run(jobCtx, request, func(event application.Event) {
				if event.Kind == "job_completed" || event.Kind == "job_failed" || event.Kind == "job_cancelled" {
					terminalEmitted = true
				}
				data := map[string]any{}
				if event.Job != nil {
					data["job"] = event.Job
				}
				if event.Progress != nil {
					data["progress"] = event.Progress
				}
				if event.Summary != nil {
					data["summary"] = event.Summary
				}
				if event.Message != "" {
					data["message"] = event.Message
				}
				if len(data) == 0 {
					data["message"] = ""
				}
				emit(protocol.Message{ID: req.ID, Event: event.Kind, Data: data})
			})
			if err != nil && !terminalEmitted {
				emitError(emit, req.ID, "start_download_failed", err)
			}
		}()
	case "resume_download":
		if h.Downloads == nil {
			emitError(emit, req.ID, "unavailable", fmt.Errorf("download service is not configured"))
			return
		}
		var p struct {
			JobID string `json:"jobId"`
		}
		if err := decodeParams(req.Params, &p); err != nil || strings.TrimSpace(p.JobID) == "" {
			if err == nil {
				err = fmt.Errorf("jobId is required")
			}
			emitError(emit, req.ID, "invalid_params", err)
			return
		}
		if !h.requireManagedDownloadVersion(ctx, emit, req.ID) {
			return
		}
		jobCtx, cancel, beginErr := h.beginJob(ctx, p.JobID)
		if beginErr != nil {
			emitError(emit, req.ID, "duplicate_job_id", beginErr)
			return
		}
		emit(protocol.Message{ID: req.ID, Event: "job_accepted", Data: map[string]any{"jobId": p.JobID, "resuming": true}})
		go func() {
			defer h.finishJob(p.JobID, cancel)
			terminalEmitted := false
			_, err := h.Downloads.Resume(jobCtx, p.JobID, func(event application.Event) {
				if isTerminalJobEvent(event.Kind) {
					terminalEmitted = true
				}
				data := map[string]any{}
				if event.Job != nil {
					data["job"] = event.Job
				}
				if event.Progress != nil {
					data["progress"] = event.Progress
				}
				if event.Summary != nil {
					data["summary"] = event.Summary
				}
				if event.Message != "" {
					data["message"] = event.Message
				}
				emit(protocol.Message{ID: req.ID, Event: event.Kind, Data: data})
			})
			if err != nil && !terminalEmitted {
				emitError(emit, req.ID, "resume_download_failed", err)
			}
		}()
	case "export_chat":
		var p protocol.ExportChatParams
		if err := decodeParams(req.Params, &p); err != nil {
			emitError(emit, req.ID, "invalid_params", err)
			return
		}
		chat, err := chatFromSelector(p.Chat)
		if err != nil {
			emitError(emit, req.ID, "invalid_params", err)
			return
		}
		runExport := func(exportCtx context.Context) error { return h.Telegram.ExportChat(exportCtx, chat, p.OutputPath) }
		if h.Downloads != nil {
			exportCtx, cancel, err := h.beginJob(ctx, req.ID)
			if err != nil {
				emitError(emit, req.ID, "export_failed", err)
				return
			}
			go func() {
				defer h.finishJob(req.ID, cancel)
				err := h.Downloads.WithTDL(exportCtx, func() error { return runExport(exportCtx) })
				if err != nil {
					emitError(emit, req.ID, "export_failed", err)
					return
				}
				emit(protocol.Message{ID: req.ID, Event: "result", Data: map[string]any{"outputPath": p.OutputPath}})
			}()
		} else {
			if err := runExport(ctx); err != nil {
				emitError(emit, req.ID, "export_failed", err)
				return
			}
			emit(protocol.Message{ID: req.ID, Event: "result", Data: map[string]any{"outputPath": p.OutputPath}})
		}
	case "download_export":
		var p protocol.DownloadParams
		if err := decodeParams(req.Params, &p); err != nil {
			emitError(emit, req.ID, "invalid_params", err)
			return
		}
		if !h.requireManagedDownloadVersion(ctx, emit, req.ID) {
			return
		}
		jobCtx, cancel, beginErr := h.beginJob(ctx, req.ID)
		if beginErr != nil {
			emitError(emit, req.ID, "duplicate_job_id", beginErr)
			return
		}
		emit(protocol.Message{ID: req.ID, Event: "download_started", Data: map[string]any{"directory": p.Directory}})
		go func() {
			defer h.finishJob(req.ID, cancel)
			request := domain.DownloadRequest{
				JobID: req.ID, ExportPath: p.ExportPath, Destination: p.Directory,
				Filter: domain.FilterMode(p.Filter), IncludeExtensions: p.Include,
				Takeout: p.Takeout, Continue: p.Continue, GroupMedia: p.Group, SkipSame: p.SkipSame,
			}
			lastEmit := time.Time{}
			runDownload := func() error {
				return h.Telegram.Download(jobCtx, request, func(progress domain.ProgressEvent) {
					now := time.Now()
					if !lastEmit.IsZero() && now.Sub(lastEmit) < 200*time.Millisecond {
						return
					}
					lastEmit = now
					emit(protocol.Message{ID: req.ID, Event: "download_progress", Data: progress})
				})
			}
			var err error
			if h.Downloads != nil {
				err = h.Downloads.WithTDL(jobCtx, runDownload)
			} else {
				err = runDownload()
			}
			if err != nil {
				if jobCtx.Err() != nil {
					emit(protocol.Message{ID: req.ID, Event: "download_cancelled", Data: map[string]any{"message": "Download process was interrupted; safe resume behavior must be verified with the selected tdl version."}})
					return
				}
				emitError(emit, req.ID, "download_failed", err)
				return
			}
			emit(protocol.Message{ID: req.ID, Event: "download_completed", Data: map[string]any{"directory": p.Directory}})
		}()
	case "pause_download":
		var p struct {
			JobID string `json:"jobId"`
		}
		if err := decodeParams(req.Params, &p); err != nil {
			emitError(emit, req.ID, "invalid_params", err)
			return
		}
		version, versionErr := h.detectTDLVersion(ctx)
		if versionErr != nil || !h.Telegram.SafeInterruptionSupported(version) {
			message := fmt.Sprintf("safe Pause/Resume requires tdl v%s or newer and a platform with graceful process interruption", h.Telegram.MinimumSafeManagedDownloadVersion())
			if versionErr != nil {
				message += ": " + versionErr.Error()
			}
			emitError(emit, req.ID, "pause_unsupported", fmt.Errorf("%s", message))
			return
		}
		h.mu.Lock()
		cancel, ok := h.jobs[p.JobID]
		h.mu.Unlock()
		if !ok {
			emitError(emit, req.ID, "job_not_found", fmt.Errorf("no active job with ID %q", p.JobID))
			return
		}
		cancel(application.ErrPauseRequested)
		emit(protocol.Message{ID: req.ID, Event: "result", Data: map[string]any{"jobId": p.JobID, "pauseRequested": true}})
	case "cancel_download":
		var p struct {
			JobID string `json:"jobId"`
		}
		if err := decodeParams(req.Params, &p); err != nil {
			emitError(emit, req.ID, "invalid_params", err)
			return
		}
		h.mu.Lock()
		cancel, ok := h.jobs[p.JobID]
		h.mu.Unlock()
		if ok {
			cancel(application.ErrCancelRequested)
			emit(protocol.Message{ID: req.ID, Event: "result", Data: map[string]any{"jobId": p.JobID, "cancellationRequested": true}})
			return
		}
		if h.Downloads == nil {
			emitError(emit, req.ID, "job_not_found", fmt.Errorf("no active job with ID %q", p.JobID))
			return
		}
		_, err := h.Downloads.CancelPersistedJob(context.Background(), p.JobID, func(event application.Event) {
			data := map[string]any{}
			if event.Job != nil {
				data["job"] = event.Job
			}
			if event.Message != "" {
				data["message"] = event.Message
			}
			emit(protocol.Message{ID: req.ID, Event: event.Kind, Data: data})
		})
		if err != nil {
			emitError(emit, req.ID, "job_not_active", err)
			return
		}
		emit(protocol.Message{ID: req.ID, Event: "result", Data: map[string]any{"jobId": p.JobID, "cancellationRequested": false, "cancelled": true}})
	default:
		emit(protocol.Message{ID: req.ID, Event: "error", Error: &protocol.Error{Code: "unknown_action", Message: "Unknown action: " + req.Action}})
	}
}

func chatFromSelector(selector string) (domain.Chat, error) {
	selector = strings.TrimSpace(selector)
	if selector == "" {
		return domain.Chat{}, fmt.Errorf("chat selector is required")
	}
	if id, err := strconv.ParseInt(selector, 10, 64); err == nil {
		return domain.Chat{ID: id, Name: selector}, nil
	}
	return domain.Chat{Username: selector, Name: selector}, nil
}

func decodeParams(raw json.RawMessage, target any) error {
	if len(raw) == 0 {
		return fmt.Errorf("params are required")
	}
	if err := json.Unmarshal(raw, target); err != nil {
		return fmt.Errorf("invalid params: %w", err)
	}
	return nil
}

func emitError(emit func(protocol.Message), id, code string, err error) {
	message := strings.TrimSpace(err.Error())
	var appErr *domain.AppError
	if errors.As(err, &appErr) && appErr.Code != "" {
		code = string(appErr.Code)
	}
	emit(protocol.Message{ID: id, Event: "error", Error: &protocol.Error{Code: code, Message: message}})
}

// Close asks all active CLI jobs to stop when the backend is shutting down. This remains
// best-effort until the process supervisor and pause/resume semantics are validated.
func (h *Handler) beginJob(parent context.Context, jobID string) (context.Context, context.CancelCauseFunc, error) {
	jobCtx, cancel := context.WithCancelCause(parent)
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		cancel(errors.New("backend shutting down"))
		return nil, nil, fmt.Errorf("backend is shutting down")
	}
	if _, exists := h.jobs[jobID]; exists {
		cancel(errors.New("duplicate active job"))
		return nil, nil, fmt.Errorf("job ID %q is already active", jobID)
	}
	h.jobs[jobID] = cancel
	h.wg.Add(1)
	return jobCtx, cancel, nil
}

func (h *Handler) finishJob(jobID string, cancel context.CancelCauseFunc) {
	h.mu.Lock()
	delete(h.jobs, jobID)
	h.mu.Unlock()
	cancel(context.Canceled)
	h.wg.Done()
}

func (h *Handler) Close() {
	h.mu.Lock()
	h.closed = true
	cancels := make([]context.CancelCauseFunc, 0, len(h.jobs))
	for _, cancel := range h.jobs {
		cancels = append(cancels, cancel)
	}
	h.mu.Unlock()
	cause := error(application.ErrCancelRequested)
	h.mu.Lock()
	version := h.tdlVersion
	h.mu.Unlock()
	if version != "" && h.Telegram.SafeInterruptionSupported(version) {
		cause = application.ErrPauseRequested
	}
	for _, cancel := range cancels {
		cancel(cause)
	}
	h.wg.Wait()
}

func (h *Handler) refreshTDLVersion(ctx context.Context) (string, error) {
	version, err := h.Telegram.Version(ctx)
	if err != nil {
		return "", err
	}
	h.mu.Lock()
	h.tdlVersion = version
	h.mu.Unlock()
	return version, nil
}

func (h *Handler) detectTDLVersion(ctx context.Context) (string, error) {
	h.mu.Lock()
	version := h.tdlVersion
	h.mu.Unlock()
	if version != "" {
		return version, nil
	}
	version, err := h.Telegram.Version(ctx)
	if err != nil {
		return "", err
	}
	h.mu.Lock()
	if h.tdlVersion == "" {
		h.tdlVersion = version
	}
	version = h.tdlVersion
	h.mu.Unlock()
	return version, nil
}

func (h *Handler) requireManagedDownloadVersion(ctx context.Context, emit func(protocol.Message), requestID string) bool {
	version, err := h.detectTDLVersion(ctx)
	if err != nil {
		emitError(emit, requestID, "tdl_version_check_failed", fmt.Errorf("could not check the tdl version: %w", err))
		return false
	}
	if h.Telegram.ManagedDownloadsSupported(version) {
		return true
	}
	message := fmt.Sprintf("managed downloads require tdl v%s or newer; detected %q. Update tdl and reconnect before downloading", h.Telegram.MinimumSafeManagedDownloadVersion(), strings.TrimSpace(version))
	emitError(emit, requestID, "tdl_version_unsupported", fmt.Errorf("%s", message))
	return false
}

func isTerminalJobEvent(kind string) bool {
	switch kind {
	case "job_completed", "job_failed", "job_cancelled", "job_paused":
		return true
	default:
		return false
	}
}
