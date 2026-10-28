package tdl

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"example.com/telegram-media-downloader/backend/internal/domain"
	"example.com/telegram-media-downloader/backend/internal/ports"
	"example.com/telegram-media-downloader/backend/internal/progress"
)

// DomainAdapter translates between the tdl-specific CLI model and the application domain.
// It is intentionally thin: it does not own job persistence or process lifecycle policy.
type DomainAdapter struct {
	Client          Client
	ThumbnailHelper string
}

var (
	_ ports.HealthChecker   = DomainAdapter{}
	_ ports.ChatSource      = DomainAdapter{}
	_ ports.MessageExporter = DomainAdapter{}
	_ ports.Downloader      = DomainAdapter{}
)

func (a DomainAdapter) Version(ctx context.Context) (string, error) {
	return a.Client.Version(ctx)
}

// SupportsGracefulPause is propagated to the app layer so it can disable Pause on
// platforms where the subprocess cannot receive a graceful interrupt.
func (a DomainAdapter) SupportsGracefulPause() bool {
	return a.Client.SupportsGracefulPause()
}

// ManagedDownloadsSupported requires a stable tdl version with the known interrupted-
// download progress-save race fixed. The app relies on process interruption for its
// managed Cancel/Pause lifecycle, so unknown development builds fail closed.
func (DomainAdapter) ManagedDownloadsSupported(version string) bool {
	return SupportsManagedDownloadVersion(version)
}

// SafeInterruptionSupported additionally checks whether this operating system can send
// the graceful process interrupt used for Pause/Resume.
func (a DomainAdapter) SafeInterruptionSupported(version string) bool {
	return a.Client.SupportsGracefulPause() && SupportsManagedDownloadVersion(version)
}

func (DomainAdapter) MinimumSafeManagedDownloadVersion() string {
	return MinimumSafeManagedDownloadVersion
}

func (a DomainAdapter) ListChats(ctx context.Context) ([]domain.Chat, error) {
	chats, err := a.Client.ListChats(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]domain.Chat, 0, len(chats))
	for _, chat := range chats {
		result = append(result, domain.Chat{ID: chat.ID, Type: chat.Type, Name: chat.VisibleName, Username: chat.Username, Topics: chat.Topics})
	}
	return result, nil
}

func (a DomainAdapter) ExportChat(ctx context.Context, chat domain.Chat, outputPath string) error {
	if chat.ID == 0 && strings.TrimSpace(chat.Username) == "" {
		return errors.New("chat must include an ID or username")
	}
	selector := strconv.FormatInt(chat.ID, 10)
	if username := strings.TrimSpace(chat.Username); username != "" {
		selector = username
		if !strings.HasPrefix(selector, "@") && !strings.Contains(selector, "://") && !strings.HasPrefix(selector, "+") {
			selector = "@" + selector
		}
	}
	return a.Client.ExportChat(ctx, selector, outputPath)
}

func (a DomainAdapter) Download(ctx context.Context, request domain.DownloadRequest, onProgress func(domain.ProgressEvent)) error {
	if strings.TrimSpace(request.ExportPath) == "" {
		return errors.New("export path is required")
	}
	if strings.TrimSpace(request.Destination) == "" {
		return errors.New("destination directory is required")
	}
	include := append([]string(nil), request.IncludeExtensions...)
	if len(include) == 0 {
		filter := request.Filter
		if filter == "" {
			filter = domain.FilterVideosAndDocuments
		}
		var err error
		include, err = domain.ExtensionsForFilter(filter)
		if err != nil {
			return err
		}
	}
	options := DownloadOptions{
		ExportPath: request.ExportPath,
		Directory:  request.Destination,
		Include:    include,
		Takeout:    request.Takeout,
		Continue:   request.Continue,
		Group:      request.GroupMedia,
		SkipSame:   request.SkipSame,
		Template:   "{{ .DialogID }}_{{ .MessageID }}_{{ filenamify .FileName }}",
	}
	return a.Client.DownloadWithProgress(ctx, options, func(stream, line string) {
		// tdl's terminal renderer may write to stdout or stderr depending on version
		// and execution mode. Parse both streams; the parser rejects CPU/RAM/goroutine
		// diagnostics and other lines without a download percentage.
		if onProgress == nil {
			return
		}
		snapshot, ok := progress.Parse(line)
		if !ok {
			return
		}
		event := domain.ProgressEvent{JobID: request.JobID, FileName: snapshot.Label}
		event.BytesDownloaded = parseRateBytesPerSecond(snapshot.Transferred)
		if snapshot.HasPercent {
			percent := snapshot.Percent
			event.Percent = &percent
		}
		if speed := parseRateBytesPerSecond(snapshot.Speed); speed > 0 {
			event.SpeedBytesPerSecond = &speed
		}
		if eta, err := time.ParseDuration(strings.TrimSpace(snapshot.ETA)); err == nil && eta >= 0 {
			seconds := int64(eta.Seconds())
			event.ETASeconds = &seconds
		}
		onProgress(event)
	})
}

func parseRateBytesPerSecond(value string) int64 {
	fields := strings.Fields(strings.TrimSpace(value))
	if len(fields) != 2 {
		return 0
	}
	number, err := strconv.ParseFloat(fields[0], 64)
	if err != nil || number < 0 {
		return 0
	}
	unit := strings.TrimSuffix(fields[1], "/s")
	base := float64(1000)
	if strings.HasSuffix(unit, "iB") {
		base = 1024
		unit = strings.TrimSuffix(unit, "iB")
	} else {
		unit = strings.TrimSuffix(unit, "B")
	}
	factor := float64(1)
	switch strings.ToUpper(unit) {
	case "", "B":
		factor = 1
	case "K":
		factor = base
	case "M":
		factor = base * base
	case "G":
		factor = base * base * base
	case "T":
		factor = base * base * base * base
	default:
		return 0
	}
	if number*factor > float64(int64(^uint64(0)>>1)) {
		return 0
	}
	return int64(number * factor)
}

func (a DomainAdapter) ParseExport(data []byte) ([]domain.MediaItem, error) {
	items, err := ParseExport(data)
	if err != nil {
		return nil, fmt.Errorf("convert tdl export: %w", err)
	}
	return items, nil
}

func (a DomainAdapter) FilterExport(data []byte, items []domain.MediaItem) ([]byte, error) {
	return FilterExport(data, items)
}
