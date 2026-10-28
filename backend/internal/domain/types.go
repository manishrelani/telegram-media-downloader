// Package domain contains the pure application model and rules. It deliberately has no
// dependency on Telegram, tdl, Flutter, the filesystem, or terminal rendering.
package domain

import (
	"fmt"
	"strings"
)

type Chat struct {
	ID       int64   `json:"id"`
	Type     string  `json:"type"`
	Name     string  `json:"name"`
	Username string  `json:"username,omitempty"`
	Topics   []Topic `json:"topics,omitempty"`
}

type ThumbnailReference struct {
	ID            int64  `json:"id"`
	AccessHash    int64  `json:"accessHash"`
	FileReference []byte `json:"fileReference"`
	DCID          int    `json:"dcId"`
	ThumbSize     string `json:"thumbSize"`
	Size          int64  `json:"size"`
	Width         int    `json:"-"`
	Height        int    `json:"-"`
}

type PreviewRequest struct {
	Key       string             `json:"key"`
	Reference ThumbnailReference `json:"reference"`
}

type MediaItem struct {
	ContentID        string              `json:"contentId,omitempty"`
	ThumbnailVersion string              `json:"thumbnailVersion,omitempty"`
	Preview          *ThumbnailReference `json:"-"`
	HasSpoiler       bool                `json:"hasSpoiler,omitempty"`
	Thumbnail        string              `json:"thumbnail,omitempty"`
	ChatID           int64               `json:"chatId"`
	MessageID        int64               `json:"messageId"`
	Caption          string              `json:"caption,omitempty"`
	OriginalName     string              `json:"originalName"`
	SizeBytes        int64               `json:"sizeBytes,omitempty"`
	Date             int64               `json:"date,omitempty"`
	DurationSeconds  float64             `json:"durationSeconds,omitempty"`
	MimeType         string              `json:"mimeType,omitempty"`
	TopicID          int64               `json:"topicId,omitempty"`
}

// Key is stable across retries and includes the original name because one message can
// contain multiple media attachments.
func (m MediaItem) Key() string {
	return fmt.Sprintf("%d:%d:%s", m.ChatID, m.MessageID, m.OriginalName)
}

type JobState string

const (
	JobQueued      JobState = "queued"
	JobPreparing   JobState = "preparing"
	JobDownloading JobState = "downloading"
	JobPausing     JobState = "pausing"
	JobPaused      JobState = "paused"
	JobCancelling  JobState = "cancelling"
	JobCancelled   JobState = "cancelled"
	JobCompleted   JobState = "completed"
	JobFailed      JobState = "failed"
)

func (s JobState) Valid() bool {
	switch s {
	case JobQueued, JobPreparing, JobDownloading, JobPausing, JobPaused, JobCancelling, JobCancelled, JobCompleted, JobFailed:
		return true
	default:
		return false
	}
}

func CanTransition(from, to JobState) bool {
	if !from.Valid() || !to.Valid() || from == to {
		return false
	}
	switch from {
	case JobQueued:
		return to == JobPreparing || to == JobPausing || to == JobCancelled
	case JobPreparing:
		return to == JobDownloading || to == JobCompleted || to == JobPausing || to == JobCancelling || to == JobFailed
	case JobDownloading:
		return to == JobPausing || to == JobCancelling || to == JobCompleted || to == JobFailed
	case JobPausing:
		return to == JobPaused || to == JobCancelling || to == JobFailed
	case JobPaused:
		return to == JobPreparing || to == JobCancelling || to == JobCancelled
	case JobCancelling:
		return to == JobCancelled || to == JobFailed
	case JobFailed, JobCancelled:
		return to == JobQueued
	case JobCompleted:
		return false
	default:
		return false
	}
}

func Transition(from, to JobState) (JobState, error) {
	if !CanTransition(from, to) {
		return from, fmt.Errorf("invalid download job state transition %q -> %q", from, to)
	}
	return to, nil
}

// DownloadJob is the persisted aggregate for a chat download. Runtime-specific process
// identifiers are intentionally kept outside this domain type.
type DownloadJob struct {
	WorkDirectory   string          `json:"workDirectory,omitempty"`
	Library         bool            `json:"library,omitempty"`
	DestinationRoot string          `json:"destinationRoot,omitempty"`
	Topic           *Topic          `json:"topic,omitempty"`
	FileTypes       []string        `json:"fileTypes,omitempty"`
	Files           []FileTask      `json:"files,omitempty"`
	Selection       *MediaSelection `json:"selection,omitempty"`
	ID              string          `json:"id"`
	Chat            Chat            `json:"chat"`
	State           JobState        `json:"state"`
	Destination     string          `json:"destination"`
	Filter          FilterMode      `json:"filter"`
	Items           []MediaItem     `json:"items,omitempty"`
	Takeout         bool            `json:"takeout,omitempty"`
	GroupMedia      bool            `json:"groupMedia,omitempty"`
	SkipSame        bool            `json:"skipSame,omitempty"`
	ErrorCode       string          `json:"errorCode,omitempty"`
	ErrorMessage    string          `json:"errorMessage,omitempty"`
}

func (j DownloadJob) Transition(to JobState) (DownloadJob, error) {
	next, err := Transition(j.State, to)
	if err != nil {
		return j, err
	}
	j.State = next
	return j, nil
}

func CleanText(s string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(s)), " ")
}

type DownloadRequest struct {
	JobID             string     `json:"jobId"`
	Chat              Chat       `json:"chat"`
	ExportPath        string     `json:"exportPath"`
	Destination       string     `json:"destination"`
	Filter            FilterMode `json:"filter"`
	IncludeExtensions []string   `json:"includeExtensions,omitempty"`
	Takeout           bool       `json:"takeout,omitempty"`
	Continue          bool       `json:"continue,omitempty"`
	GroupMedia        bool       `json:"groupMedia,omitempty"`
	SkipSame          bool       `json:"skipSame,omitempty"`
}

type ProgressEvent struct {
	ItemKey             string   `json:"itemKey,omitempty"`
	UpdatedAt           int64    `json:"updatedAt,omitempty"`
	JobID               string   `json:"jobId"`
	FileName            string   `json:"fileName,omitempty"`
	BytesDownloaded     int64    `json:"bytesDownloaded,omitempty"`
	TotalBytes          int64    `json:"totalBytes,omitempty"`
	Percent             *float64 `json:"percent,omitempty"`
	SpeedBytesPerSecond *int64   `json:"speedBytesPerSecond,omitempty"`
	ETASeconds          *int64   `json:"etaSeconds,omitempty"`
}

type ErrorCode string

const (
	ErrorCodeInvalidInput   ErrorCode = "invalid_input"
	ErrorCodeNotFound       ErrorCode = "not_found"
	ErrorCodeUnavailable    ErrorCode = "unavailable"
	ErrorCodeUnauthorized   ErrorCode = "unauthorized"
	ErrorCodeRateLimited    ErrorCode = "rate_limited"
	ErrorCodeStorage        ErrorCode = "storage_error"
	ErrorCodeDownloadFailed ErrorCode = "download_failed"
	ErrorCodeInterrupted    ErrorCode = "interrupted"
	ErrorCodeInternal       ErrorCode = "internal_error"
)

type AppError struct {
	Code      ErrorCode
	Message   string
	Retryable bool
	Cause     error
}

func (e *AppError) Error() string {
	if e == nil {
		return ""
	}
	if e.Message != "" {
		return e.Message
	}
	if e.Cause != nil {
		return e.Cause.Error()
	}
	return string(e.Code)
}

func (e *AppError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

func (j DownloadJob) Validate() error {
	if strings.TrimSpace(j.ID) == "" {
		return fmt.Errorf("job ID is required")
	}
	if !j.State.Valid() {
		return fmt.Errorf("invalid job state %q", j.State)
	}
	if strings.TrimSpace(j.Destination) == "" {
		return fmt.Errorf("job destination is required")
	}
	if !j.Filter.Valid() {
		return fmt.Errorf("invalid job filter %q", j.Filter)
	}
	return nil
}
