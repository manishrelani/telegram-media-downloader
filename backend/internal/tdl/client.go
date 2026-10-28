package tdl

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"example.com/telegram-media-downloader/backend/internal/domain"
)

type Executor interface {
	Run(context.Context, ...string) (stdout []byte, stderr []byte, err error)
}

type OSExecutor struct {
	Binary              string
	PreviewProcessGroup bool
}

func (e OSExecutor) Run(ctx context.Context, args ...string) ([]byte, []byte, error) {
	binary := strings.TrimSpace(e.Binary)
	if binary == "" {
		binary = "tdl"
	}
	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.WaitDelay = 5 * time.Second
	if e.PreviewProcessGroup {
		configurePreviewProcess(cmd)
	}
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return os.ErrProcessDone
		}
		if e.PreviewProcessGroup {
			return interruptPreviewProcess(cmd.Process)
		}
		return interruptProcess(cmd.Process)
	}
	stdout, err := cmd.Output()
	var stderr []byte
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			stderr = append(stderr, ee.Stderr...)
		}
	}
	return stdout, stderr, err
}

// RunStream preserves carriage-return updates emitted by terminal-oriented CLIs.
// It is a subprocess adapter only; the output format remains an upstream dependency.
func (e OSExecutor) RunStream(ctx context.Context, args []string, onOutput func(stream, text string)) error {
	binary := strings.TrimSpace(e.Binary)
	if binary == "" {
		binary = "tdl"
	}
	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.WaitDelay = 5 * time.Second
	if e.PreviewProcessGroup {
		configurePreviewProcess(cmd)
	}
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return os.ErrProcessDone
		}
		if e.PreviewProcessGroup {
			return interruptPreviewProcess(cmd.Process)
		}
		return interruptProcess(cmd.Process)
	}

	var stderrMu sync.Mutex
	var stderrText strings.Builder
	var callbackMu sync.Mutex
	emitOutput := func(stream, text string) {
		if onOutput == nil {
			return
		}
		callbackMu.Lock()
		defer callbackMu.Unlock()
		onOutput(stream, text)
	}
	stdoutWriter := &segmentOutputWriter{emit: func(line string) { emitOutput("stdout", line) }}
	stderrWriter := &segmentOutputWriter{emit: func(line string) {
		stderrMu.Lock()
		stderrText.WriteString(line)
		stderrText.WriteByte('\n')
		if stderrText.Len() > 64*1024 {
			tail := stderrText.String()
			stderrText.Reset()
			stderrText.WriteString(tail[len(tail)-32*1024:])
		}
		stderrMu.Unlock()
		emitOutput("stderr", line)
	}}
	cmd.Stdout = stdoutWriter
	cmd.Stderr = stderrWriter

	waitErr := cmd.Run()
	stdoutWriter.Flush()
	stderrWriter.Flush()
	if waitErr != nil {
		stderrMu.Lock()
		detail := []byte(stderrText.String())
		stderrMu.Unlock()
		return commandError("tdl command", detail, waitErr)
	}
	return nil
}

// SupportsGracefulPause reports whether this platform can send a graceful interrupt
// to tdl. It is deliberately separate from CLI availability: Windows requires a
// dedicated console-control implementation before Pause can safely be offered.
func (Client) SupportsGracefulPause() bool { return supportsGracefulInterrupt() }

type segmentOutputWriter struct {
	mu     sync.Mutex
	buffer []byte
	emit   func(string)
}

func (w *segmentOutputWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	segments := make([]string, 0, 4)
	for _, b := range p {
		if b == '\r' || b == '\n' {
			if len(w.buffer) > 0 {
				segments = append(segments, string(w.buffer))
				w.buffer = w.buffer[:0]
			}
			continue
		}
		w.buffer = append(w.buffer, b)
		if len(w.buffer) >= 1024*1024 {
			segments = append(segments, string(w.buffer))
			w.buffer = w.buffer[:0]
		}
	}
	w.mu.Unlock()
	for _, segment := range segments {
		if w.emit != nil {
			w.emit(segment)
		}
	}
	return len(p), nil
}

func (w *segmentOutputWriter) Flush() {
	w.mu.Lock()
	if len(w.buffer) == 0 {
		w.mu.Unlock()
		return
	}
	segment := string(w.buffer)
	w.buffer = w.buffer[:0]
	w.mu.Unlock()
	if w.emit != nil {
		w.emit(segment)
	}
}

// readSegments treats both carriage returns and newlines as progress boundaries. tdl uses
// carriage-return updates in its terminal progress display, so Scanner's default newline
// delimiter would otherwise buffer many status updates into one unreadable blob.
func readSegments(r io.Reader, consume func(string)) error {
	reader := bufio.NewReader(r)
	var segment strings.Builder
	for {
		b, err := reader.ReadByte()
		if err != nil {
			if segment.Len() > 0 {
				consume(segment.String())
			}
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
		if b == '\r' || b == '\n' {
			if segment.Len() > 0 {
				consume(segment.String())
				segment.Reset()
			}
			continue
		}
		segment.WriteByte(b)
		if segment.Len() >= 1024*1024 {
			consume(segment.String())
			segment.Reset()
		}
	}
}

type StreamExecutor interface {
	RunStream(context.Context, []string, func(stream, text string)) error
}

type Client struct{ Exec Executor }

type Chat struct {
	ID          int64          `json:"id"`
	Type        string         `json:"type"`
	VisibleName string         `json:"visible_name"`
	Username    string         `json:"username,omitempty"`
	Topics      []domain.Topic `json:"topics,omitempty"`
}

func (c Client) executor() (Executor, error) {
	if c.Exec == nil {
		return nil, errors.New("tdl executor is not configured")
	}
	return c.Exec, nil
}

func (c Client) Version(ctx context.Context) (string, error) {
	e, err := c.executor()
	if err != nil {
		return "", err
	}
	out, stderr, err := e.Run(ctx, "version")
	if err != nil {
		return "", commandError("tdl version", stderr, err)
	}
	return strings.TrimSpace(string(out)), nil
}

// ListChats performs an authenticated, non-destructive operation. A failure here may
// mean the session is not logged in, the network is unavailable, or tdl returned an error;
// callers should display the returned detail instead of automatically assuming login expired.
func (c Client) ListChats(ctx context.Context) ([]Chat, error) {
	e, err := c.executor()
	if err != nil {
		return nil, err
	}
	out, stderr, err := e.Run(ctx, "chat", "ls", "-o", "json")
	if err != nil {
		return nil, commandError("tdl chat ls -o json", stderr, err)
	}
	var chats []Chat
	if err := json.Unmarshal(out, &chats); err != nil {
		return nil, fmt.Errorf("parse tdl chat list JSON: %w", err)
	}
	if chats == nil {
		chats = []Chat{}
	}
	return chats, nil
}

func (c Client) ExportChat(ctx context.Context, chat, outputPath string) error {
	if strings.TrimSpace(chat) == "" {
		return errors.New("chat is required")
	}
	if strings.TrimSpace(outputPath) == "" {
		return errors.New("output path is required")
	}
	e, err := c.executor()
	if err != nil {
		return err
	}
	_, stderr, err := e.Run(ctx, "chat", "export", "-c", chat, "--with-content", "--raw", "-o", outputPath)
	if err != nil {
		return commandError("tdl chat export", stderr, err)
	}
	return nil
}

type DownloadOptions struct {
	ExportPath string
	Directory  string
	Include    []string
	Takeout    bool
	Continue   bool
	Group      bool
	SkipSame   bool
	Template   string
}

func BuildDownloadArgs(o DownloadOptions) ([]string, error) {
	if strings.TrimSpace(o.ExportPath) == "" {
		return nil, errors.New("export path is required")
	}
	if strings.TrimSpace(o.Directory) == "" {
		return nil, errors.New("destination directory is required")
	}
	args := []string{"dl", "-f", o.ExportPath, "-d", o.Directory}
	if len(o.Include) > 0 {
		args = append(args, "-i", strings.Join(o.Include, ","))
	}
	if o.Template != "" {
		args = append(args, "--template", o.Template)
	}
	if o.Takeout {
		args = append(args, "--takeout")
	}
	if o.Continue {
		args = append(args, "--continue")
	}
	if o.Group {
		args = append(args, "--group")
	}
	if o.SkipSame {
		args = append(args, "--skip-same")
	}
	return args, nil
}

func (c Client) Download(ctx context.Context, o DownloadOptions) error {
	return c.DownloadWithProgress(ctx, o, nil)
}

func (c Client) DownloadWithProgress(ctx context.Context, o DownloadOptions, onOutput func(stream, text string)) error {
	args, err := BuildDownloadArgs(o)
	if err != nil {
		return err
	}
	e, err := c.executor()
	if err != nil {
		return err
	}
	if stream, ok := e.(StreamExecutor); ok {
		if err := stream.RunStream(ctx, args, onOutput); err != nil {
			return fmt.Errorf("tdl download failed: %w", err)
		}
		return nil
	}
	_, stderr, err := e.Run(ctx, args...)
	if err != nil {
		return commandError("tdl download", stderr, err)
	}
	return nil
}

func commandError(operation string, stderr []byte, err error) error {
	detail := strings.TrimSpace(string(stderr))
	if detail == "" && err != nil {
		detail = err.Error()
	}
	lower := strings.ToLower(operation + " " + detail)
	appErr := &domain.AppError{Code: domain.ErrorCodeInternal, Retryable: false, Cause: err}

	var execErr *exec.Error
	if errors.As(err, &execErr) && errors.Is(execErr.Err, exec.ErrNotFound) {
		appErr.Code = domain.ErrorCodeUnavailable
		appErr.Message = "The Telegram runtime was not found. Reinstall the app or rebuild its bundled runtime; TDL_PATH is a trusted developer override."
		return appErr
	}

	switch {
	case strings.Contains(lower, "current database is used by another process"),
		strings.Contains(lower, "database is locked"), strings.Contains(lower, "database locked"):
		appErr.Code = domain.ErrorCodeStorage
		appErr.Retryable = true
		appErr.Message = "The tdl database is used by another process. Close the other tdl process and retry."
	case strings.Contains(lower, "flood_wait"), strings.Contains(lower, "flood_premium_wait"), strings.Contains(lower, "rate limit"):
		appErr.Code = domain.ErrorCodeRateLimited
		appErr.Retryable = true
		appErr.Message = "Telegram rate-limited this operation. Wait before retrying."
	case strings.Contains(lower, "auth_key_unregistered"), strings.Contains(lower, "session revoked"),
		strings.Contains(lower, "unauthorized"), strings.Contains(lower, "not authorized"),
		strings.Contains(lower, "not logged in"):
		appErr.Code = domain.ErrorCodeUnauthorized
		appErr.Retryable = false
		appErr.Message = "Telegram authentication failed or this account cannot access the chat. Check the session with the bundled tdl-tmm login command and verify chat access."
	case strings.Contains(lower, "permission denied"), strings.Contains(lower, "no space left on device"),
		strings.Contains(lower, "read-only file system"):
		appErr.Code = domain.ErrorCodeStorage
		appErr.Retryable = false
		appErr.Message = "The destination or tdl storage is not writable. Check permissions and available disk space."
	case strings.Contains(lower, "connection refused"), strings.Contains(lower, "network is unreachable"),
		strings.Contains(lower, "no route to host"), strings.Contains(lower, "i/o timeout"),
		strings.Contains(lower, "deadline exceeded"), strings.Contains(lower, "temporary failure in name resolution"):
		appErr.Code = domain.ErrorCodeUnavailable
		appErr.Retryable = true
		appErr.Message = "The network or Telegram service is temporarily unavailable. Check connectivity and retry."
	default:
		if strings.Contains(strings.ToLower(operation), "download") {
			appErr.Code = domain.ErrorCodeDownloadFailed
		}
		appErr.Message = operation + " failed"
	}
	if detail != "" && !strings.Contains(appErr.Message, detail) {
		// Keep diagnostics bounded so an unexpectedly large CLI error cannot flood the UI.
		if len(detail) > 500 {
			detail = detail[:500] + "…"
		}
		appErr.Message += ": " + detail
	}
	return appErr
}
