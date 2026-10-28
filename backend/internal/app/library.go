package app

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"example.com/telegram-media-downloader/backend/internal/application"
	"example.com/telegram-media-downloader/backend/internal/domain"
	"example.com/telegram-media-downloader/backend/internal/protocol"
)

func (h *Handler) handleMediaRequest(ctx context.Context, req protocol.Request, emit func(protocol.Message)) {
	if h.Downloads == nil {
		emitError(emit, req.ID, "unavailable", fmt.Errorf("download service is unavailable"))
		return
	}
	switch req.Action {
	case "cancel_media_request":
		var params struct {
			RequestID string `json:"requestId"`
		}
		if err := decodeParams(req.Params, &params); err != nil {
			emitError(emit, req.ID, "invalid_params", err)
			return
		}
		h.mu.Lock()
		cancel := h.jobs[params.RequestID]
		h.mu.Unlock()
		if cancel != nil {
			cancel(context.Canceled)
		}
		emit(protocol.Message{ID: req.ID, Event: "result"})
	case "list_media":
		var query domain.MediaQuery
		if err := decodeParams(req.Params, &query); err != nil {
			emitError(emit, req.ID, "invalid_params", err)
			return
		}
		readCtx, cancel, err := h.beginJob(ctx, req.ID)
		if err != nil {
			emitError(emit, req.ID, "request_failed", err)
			return
		}
		go func() {
			defer h.finishJob(req.ID, cancel)
			page, err := h.Downloads.ListMedia(readCtx, query)
			if err != nil {
				emitError(emit, req.ID, "media_list_failed", err)
				return
			}
			emit(protocol.Message{ID: req.ID, Event: "result", Data: page})
		}()
	case "request_thumbnails":
		var params struct {
			SnapshotID string   `json:"snapshotId"`
			Keys       []string `json:"keys"`
		}
		if err := decodeParams(req.Params, &params); err != nil || params.SnapshotID == "" || len(params.Keys) == 0 || len(params.Keys) > 12 {
			emitError(emit, req.ID, "invalid_params", fmt.Errorf("snapshotId and 1–12 visible keys are required"))
			return
		}
		previewCtx, cancel, err := h.beginJob(ctx, req.ID)
		if err != nil {
			emitError(emit, req.ID, "request_failed", err)
			return
		}
		go func() {
			defer h.finishJob(req.ID, cancel)
			busy, err := h.Downloads.RequestThumbnails(previewCtx, params.SnapshotID, params.Keys, func(images map[string]string) {
				emit(protocol.Message{ID: req.ID, Event: "media_thumbnails", Data: map[string]any{"snapshotId": params.SnapshotID, "thumbnails": images}})
			})
			if err != nil && previewCtx.Err() == nil {
				emitError(emit, req.ID, "thumbnail_unavailable", err)
				return
			}
			emit(protocol.Message{ID: req.ID, Event: "result", Data: map[string]any{"busy": busy, "cancelled": previewCtx.Err() != nil}})
		}()

	case "start_media_download":
		var params struct {
			Chat            domain.Chat           `json:"chat"`
			DestinationRoot string                `json:"destinationRoot"`
			Selection       domain.MediaSelection `json:"selection"`
			Takeout         bool                  `json:"takeout"`
		}
		if err := decodeParams(req.Params, &params); err != nil {
			emitError(emit, req.ID, "invalid_params", err)
			return
		}
		if !h.requireManagedDownloadVersion(ctx, emit, req.ID) {
			return
		}
		jobCtx, cancel, err := h.beginJob(ctx, req.ID)
		if err != nil {
			emitError(emit, req.ID, "duplicate_job_id", err)
			return
		}
		emit(protocol.Message{ID: req.ID, Event: "job_accepted", Data: map[string]any{"jobId": req.ID}})
		go func() {
			defer h.finishJob(req.ID, cancel)
			terminal := false
			_, err := h.Downloads.RunMedia(jobCtx, application.MediaRunRequest{JobID: req.ID, Chat: params.Chat, DestinationRoot: params.DestinationRoot, Selection: params.Selection, Takeout: params.Takeout}, h.mediaEmitter(req.ID, emit, &terminal))
			if err != nil && !terminal {
				emitError(emit, req.ID, "start_download_failed", err)
			}
		}()
	case "retry_download":
		var params struct {
			JobID string `json:"jobId"`
		}
		if err := decodeParams(req.Params, &params); err != nil || params.JobID == "" {
			emitError(emit, req.ID, "invalid_params", fmt.Errorf("jobId is required"))
			return
		}
		if !h.requireManagedDownloadVersion(ctx, emit, req.ID) {
			return
		}
		jobCtx, cancel, err := h.beginJob(ctx, params.JobID)
		if err != nil {
			emitError(emit, req.ID, "duplicate_job_id", err)
			return
		}
		emit(protocol.Message{ID: req.ID, Event: "job_accepted", Data: map[string]any{"jobId": params.JobID}})
		go func() {
			defer h.finishJob(params.JobID, cancel)
			terminal := false
			_, err := h.Downloads.RetryMedia(jobCtx, params.JobID, h.mediaEmitter(req.ID, emit, &terminal))
			if err != nil && !terminal {
				emitError(emit, req.ID, "retry_failed", err)
			}
		}()
	case "open_download_file":
		var params struct {
			JobID   string `json:"jobId"`
			ItemKey string `json:"itemKey"`
			Reveal  bool   `json:"reveal"`
		}
		if err := decodeParams(req.Params, &params); err != nil {
			emitError(emit, req.ID, "invalid_params", err)
			return
		}
		job, err := h.Downloads.Jobs.Get(ctx, params.JobID)
		if err != nil {
			emitError(emit, req.ID, "not_found", err)
			return
		}
		var savedPath string
		for _, file := range job.Files {
			if file.Item.Key() == params.ItemKey && file.State == "completed" {
				validated, err := application.ValidateSavedDownload(job, file)
				if err != nil {
					emitError(emit, req.ID, "unsafe_saved_file", err)
					return
				}
				savedPath = validated
				break
			}
		}
		if savedPath == "" {
			emitError(emit, req.ID, "not_found", fmt.Errorf("completed file not found"))
			return
		}
		info, err := os.Stat(savedPath)
		if err != nil || !info.Mode().IsRegular() {
			emitError(emit, req.ID, "not_found", fmt.Errorf("downloaded file is no longer available"))
			return
		}
		var command *exec.Cmd
		switch runtime.GOOS {
		case "darwin":
			args := []string{savedPath}
			if params.Reveal {
				args = []string{"-R", savedPath}
			}
			command = exec.CommandContext(ctx, "/usr/bin/open", args...)
		case "windows":
			if params.Reveal {
				command = exec.CommandContext(ctx, "explorer.exe", "/select,", savedPath)
			} else {
				command = exec.CommandContext(ctx, "rundll32.exe", "url.dll,FileProtocolHandler", savedPath)
			}
		default:
			if params.Reveal {
				savedPath = filepath.Dir(savedPath)
			}
			command = exec.CommandContext(ctx, "xdg-open", savedPath)
		}
		if err := command.Run(); err != nil {
			emitError(emit, req.ID, "open_failed", err)
			return
		}
		emit(protocol.Message{ID: req.ID, Event: "result"})
	}
}

func (h *Handler) mediaEmitter(id string, emit func(protocol.Message), terminal *bool) func(application.Event) {
	return func(event application.Event) {
		if isTerminalJobEvent(event.Kind) {
			*terminal = true
		}
		data := map[string]any{}
		if event.Job != nil {
			data["job"] = event.Job
		}
		if event.Progress != nil {
			data["progress"] = event.Progress
		}
		if event.Message != "" {
			data["message"] = event.Message
		}
		emit(protocol.Message{ID: id, Event: event.Kind, Data: data})
	}
}
