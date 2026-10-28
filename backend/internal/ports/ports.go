// Package ports declares application-facing interfaces. Adapters (tdl, filesystem, and
// persistent storage) implement these contracts; use cases depend on the interfaces only.
package ports

import (
	"context"
	"errors"

	"example.com/telegram-media-downloader/backend/internal/domain"
)

// ManagedDownloadCapabilities reports which lifecycle guarantees an adapter supports
// for a detected CLI version. A desktop UI should not advertise pause/resume when the
// underlying downloader version is unknown or does not preserve interrupted progress.
type ManagedDownloadCapabilities interface {
	ManagedDownloadsSupported(version string) bool
	SafeInterruptionSupported(version string) bool
	MinimumSafeManagedDownloadVersion() string
}

type HealthChecker interface {
	Version(ctx context.Context) (string, error)
}

type ChatSource interface {
	ListChats(ctx context.Context) ([]domain.Chat, error)
}

type MessageExporter interface {
	ExportChat(ctx context.Context, chat domain.Chat, outputPath string) error
}

type Downloader interface {
	Download(ctx context.Context, request domain.DownloadRequest, onProgress func(domain.ProgressEvent)) error
}

type FileInfo struct {
	Name    string
	Size    int64
	IsDir   bool
	ModTime int64
}

type FileSystem interface {
	EnsureDirectory(ctx context.Context, path string) error
	ListDirectory(ctx context.Context, path string) ([]FileInfo, error)
	Exists(ctx context.Context, path string) (bool, error)
	Rename(ctx context.Context, oldPath, newPath string) error
	AvailableBytes(ctx context.Context, path string) (uint64, error)
}

type JobStore interface {
	Save(ctx context.Context, job domain.DownloadJob) error
	Get(ctx context.Context, jobID string) (domain.DownloadJob, error)
	List(ctx context.Context) ([]domain.DownloadJob, error)
	Delete(ctx context.Context, jobID string) error
}

// ErrJobNotFound is returned by a JobStore when a requested job does not exist.
var ErrJobNotFound = errors.New("download job not found")

// ExportProcessor parses tdl's exported JSON and creates a subset containing only the
// requested media items. Implementations should preserve unknown upstream fields where possible.
type ExportProcessor interface {
	ParseExport(data []byte) ([]domain.MediaItem, error)
	FilterExport(data []byte, items []domain.MediaItem) ([]byte, error)
}

type DownloadOrganizer interface {
	OrganizeDownloads(stagingDir, chatRoot string, items []domain.MediaItem) (domain.OrganizationSummary, error)
}

// TransactionalDownloadOrganizer records the intended destination before each staging
// file is moved, allowing crash recovery to reconcile an in-progress manifest entry.
type TransactionalDownloadOrganizer interface {
	OrganizeDownloadsWithHook(stagingDir, chatRoot string, items []domain.MediaItem, beforeMove func(domain.MediaItem, string) error) (domain.OrganizationSummary, error)
}

type ManifestStore interface {
	Load(ctx context.Context, path string, chatID int64, chatName string) (*domain.Manifest, error)
	Save(ctx context.Context, path string, manifest *domain.Manifest) error
}

// TelegramGateway is the application-facing contract implemented by a Telegram/tdl adapter.
type TelegramGateway interface {
	HealthChecker
	ChatSource
	MessageExporter
	Downloader
}

// ConfinedDownloadOrganizer anchors every filesystem operation at the selected root.
type ConfinedDownloadOrganizer interface {
	OrganizeDownloadsWithin(staging, output, allowed string, items []domain.MediaItem, hook func(domain.MediaItem, string) error) (domain.OrganizationSummary, error)
}
