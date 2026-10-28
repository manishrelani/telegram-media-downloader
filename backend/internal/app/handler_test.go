package app

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"example.com/telegram-media-downloader/backend/internal/application"
	"example.com/telegram-media-downloader/backend/internal/domain"
	"example.com/telegram-media-downloader/backend/internal/filesystem"
	"example.com/telegram-media-downloader/backend/internal/ports"
	"example.com/telegram-media-downloader/backend/internal/protocol"
	"example.com/telegram-media-downloader/backend/internal/store"
	"example.com/telegram-media-downloader/backend/internal/tdl"
)

type fakeTelegram struct {
	version     string
	chats       []domain.Chat
	versionErr  error
	safePause   bool
	chatErr     error
	exportCalls []string
	exportErr   error
	downloadErr error
}

func (f *fakeTelegram) Version(context.Context) (string, error) { return f.version, f.versionErr }
func (f *fakeTelegram) ManagedDownloadsSupported(version string) bool {
	return tdl.SupportsManagedDownloadVersion(version)
}
func (f *fakeTelegram) SafeInterruptionSupported(version string) bool {
	return f.safePause && tdl.SupportsManagedDownloadVersion(version)
}
func (f *fakeTelegram) MinimumSafeManagedDownloadVersion() string {
	return tdl.MinimumSafeManagedDownloadVersion
}
func (f *fakeTelegram) ListChats(context.Context) ([]domain.Chat, error) { return f.chats, f.chatErr }
func (f *fakeTelegram) ExportChat(_ context.Context, chat domain.Chat, path string) error {
	selector := chat.Username
	if selector == "" {
		selector = fmt.Sprint(chat.ID)
	}
	f.exportCalls = []string{selector, path}
	return f.exportErr
}
func (f *fakeTelegram) Download(_ context.Context, request domain.DownloadRequest, cb func(domain.ProgressEvent)) error {
	if cb != nil {
		percent := 42.5
		cb(domain.ProgressEvent{JobID: request.JobID, FileName: "Clip.mp4", Percent: &percent})
	}
	return f.downloadErr
}

func TestHandlerHealthAndListChats(t *testing.T) {
	fake := &fakeTelegram{version: "Version: 0.20.4", chats: []domain.Chat{{ID: 7, Type: "channel", Name: "Example"}}}
	h := New(fake)
	var got []protocol.Message
	emit := func(m protocol.Message) { got = append(got, m) }
	h.Handle(context.Background(), protocol.Request{Version: 1, ID: "h1", Action: "health"}, emit)
	h.Handle(context.Background(), protocol.Request{Version: 1, ID: "c1", Action: "list_chats"}, emit)
	if len(got) != 2 || got[0].Event != "result" || got[1].Event != "result" {
		t.Fatalf("unexpected messages: %#v", got)
	}
	if !strings.Contains(mustJSON(t, got[0].Data), "0.20.4") {
		t.Fatalf("health missing version: %#v", got[0])
	}
	if !strings.Contains(mustJSON(t, got[1].Data), "Example") {
		t.Fatalf("chat list missing chat: %#v", got[1])
	}
}

func TestHandlerRejectsUnknownAction(t *testing.T) {
	h := New(&fakeTelegram{})
	var got protocol.Message
	h.Handle(context.Background(), protocol.Request{Version: 1, ID: "x", Action: "surprise"}, func(m protocol.Message) { got = m })
	if got.Error == nil || got.Error.Code != "unknown_action" {
		t.Fatalf("unexpected response: %#v", got)
	}
}

func TestHandlerExportParsesNumericChatSelector(t *testing.T) {
	fake := &fakeTelegram{}
	h := New(fake)
	var got protocol.Message
	h.Handle(context.Background(), protocol.Request{Version: 1, ID: "e1", Action: "export_chat", Params: json.RawMessage(`{"chat":"-100123","outputPath":"/tmp/export.json"}`)}, func(m protocol.Message) { got = m })
	if got.Event != "result" {
		t.Fatalf("response = %#v", got)
	}
	if len(fake.exportCalls) != 2 || fake.exportCalls[0] != "-100123" {
		t.Fatalf("export selector = %#v, want numeric ID", fake.exportCalls)
	}
}

func TestHandlerExportParsesUsernameSelector(t *testing.T) {
	fake := &fakeTelegram{}
	h := New(fake)
	var got protocol.Message
	h.Handle(context.Background(), protocol.Request{Version: 1, ID: "e2", Action: "export_chat", Params: json.RawMessage(`{"chat":"@chaicode","outputPath":"/tmp/export.json"}`)}, func(m protocol.Message) { got = m })
	if got.Event != "result" {
		t.Fatalf("response = %#v", got)
	}
	if fake.exportCalls[0] != "@chaicode" {
		t.Fatalf("selector = %q", fake.exportCalls[0])
	}
}

func TestHandlerDownloadEmitsStartProgressAndCompletion(t *testing.T) {
	fake := &fakeTelegram{version: "Version: 0.20.4"}
	h := New(fake)
	messages := make(chan protocol.Message, 10)
	h.Handle(context.Background(), protocol.Request{Version: 1, ID: "job-1", Action: "download_export", Params: json.RawMessage(`{"exportPath":"/tmp/e.json","directory":"/tmp/out","filter":"videos_and_documents"}`)}, func(m protocol.Message) { messages <- m })
	seen := map[string]bool{}
	deadline := time.After(2 * time.Second)
	for !seen["download_completed"] {
		select {
		case m := <-messages:
			seen[m.Event] = true
		case <-deadline:
			t.Fatalf("timed out waiting for completion; events=%v", seen)
		}
	}
	if !seen["download_started"] || !seen["download_progress"] {
		t.Fatalf("expected start/progress/completion, got %v", seen)
	}
}

func TestHandlerRejectsEmptyChatSelector(t *testing.T) {
	fake := &fakeTelegram{}
	h := New(fake)
	var got protocol.Message
	h.Handle(context.Background(), protocol.Request{Version: 1, ID: "e3", Action: "export_chat", Params: json.RawMessage(`{"chat":" ","outputPath":"/tmp/export.json"}`)}, func(m protocol.Message) { got = m })
	if got.Error == nil || got.Error.Code != "invalid_params" {
		t.Fatalf("response = %#v", got)
	}
}

func TestHandlerPauseRequiresGracefulBackendCapability(t *testing.T) {
	h := New(&fakeTelegram{})
	var got protocol.Message
	h.Handle(context.Background(), protocol.Request{Version: 1, ID: "pause-test", Action: "pause_download", Params: json.RawMessage(`{"jobId":"job-1"}`)}, func(m protocol.Message) { got = m })
	if got.Event != "error" || got.Error == nil || got.Error.Code != "pause_unsupported" {
		t.Fatalf("pause response = %#v, want pause_unsupported", got)
	}
}

func TestHandlerHealthReportsGracefulPauseCapability(t *testing.T) {
	h := New(&fakeTelegram{version: "tdl test"})
	var got protocol.Message
	h.Handle(context.Background(), protocol.Request{Version: 1, ID: "health-capability", Action: "health"}, func(m protocol.Message) { got = m })
	payload := mustJSON(t, got.Data)
	if !strings.Contains(payload, `"supportsGracefulPause":false`) {
		t.Fatalf("health response does not expose pause capability: %s", payload)
	}
}

func TestHandlerHealthVersionGatesManagedDownloadsAndPause(t *testing.T) {
	fake := &fakeTelegram{version: "Version: 0.20.3", safePause: true}
	h := New(fake)
	var got protocol.Message
	h.Handle(context.Background(), protocol.Request{Version: 1, ID: "health-old-version", Action: "health"}, func(m protocol.Message) { got = m })
	if got.Event != "result" {
		t.Fatalf("health response = %#v", got)
	}
	payload := mustJSON(t, got.Data)
	if !strings.Contains(payload, `"managedDownloadsSupported":false`) || !strings.Contains(payload, `"supportsGracefulPause":false`) {
		t.Fatalf("old tdl version must not advertise managed downloads or pause: %s", payload)
	}
	if !strings.Contains(payload, `"minimumSafeManagedDownloadVersion":"0.20.4"`) {
		t.Fatalf("health response missing minimum safe version: %s", payload)
	}
}

func TestHandlerHealthRefreshesTDLVersionAfterUpgrade(t *testing.T) {
	fake := &fakeTelegram{version: "Version: 0.20.3", safePause: true}
	h := New(fake)
	callHealth := func(id string) map[string]any {
		t.Helper()
		var got protocol.Message
		h.Handle(context.Background(), protocol.Request{Version: 1, ID: id, Action: "health"}, func(m protocol.Message) { got = m })
		if got.Event != "result" {
			t.Fatalf("health response = %#v", got)
		}
		data, ok := got.Data.(map[string]any)
		if !ok {
			t.Fatalf("health data has unexpected type %T", got.Data)
		}
		return data
	}
	first := callHealth("health-before-upgrade")
	if first["managedDownloadsSupported"] != false {
		t.Fatalf("old version should be unsupported, got %#v", first)
	}
	fake.version = "Version: 0.20.4"
	second := callHealth("health-after-upgrade")
	if second["managedDownloadsSupported"] != true || second["supportsGracefulPause"] != true {
		t.Fatalf("refreshed version should re-enable supported capabilities, got %#v", second)
	}
}

func TestHandlerRejectsManagedDownloadOnOldTDLVersion(t *testing.T) {
	root := t.TempDir()
	gateway := &pipelineTelegram{version: "Version: 0.20.3", exportJSON: `{"id":123,"messages":[{"id":7,"type":"message","file":"lesson.mp4","text":"Caption"}]}`}
	jobs, err := store.NewJSONJobStore(filepath.Join(t.TempDir(), "state", "jobs.json"))
	if err != nil {
		t.Fatal(err)
	}
	service := application.NewService(gateway, jobs, filesystem.DownloadOrganizer{}, filesystem.JSONManifestStore{})
	handler := NewWithDownloadService(gateway, service)
	var got protocol.Message
	handler.Handle(context.Background(), protocol.Request{
		Version: 1, ID: "old-version-job", Action: "start_chat_download",
		Params: json.RawMessage(fmt.Sprintf(`{"chat":{"id":123,"type":"channel","name":"Example"},"destinationRoot":%q,"filter":"videos_and_documents"}`, root)),
	}, func(m protocol.Message) { got = m })
	if got.Event != "error" || got.Error == nil || got.Error.Code != "tdl_version_unsupported" {
		t.Fatalf("response = %#v, want tdl_version_unsupported", got)
	}
	if !strings.Contains(got.Error.Message, "v0.20.4 or newer") {
		t.Fatalf("error should tell user how to fix it: %q", got.Error.Message)
	}
	listed, err := jobs.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(listed) != 0 {
		t.Fatalf("unsupported tdl version unexpectedly created jobs: %#v", listed)
	}
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestEmitErrorPrefersTypedDomainErrorCode(t *testing.T) {
	var got protocol.Message
	err := &domain.AppError{Code: domain.ErrorCodeRateLimited, Message: "wait before retrying", Retryable: true}
	emitError(func(message protocol.Message) { got = message }, "job-err", "download_failed", err)
	if got.Error == nil || got.Error.Code != string(domain.ErrorCodeRateLimited) {
		t.Fatalf("error response = %#v", got)
	}
}

func TestHandlerStartChatDownloadAndListJobs(t *testing.T) {
	root := t.TempDir()
	gateway := &pipelineTelegram{exportJSON: `{"id":123,"messages":[{"id":7,"type":"message","file":"lesson.mp4","text":"01. Caption Title"}]}`}
	jobs, err := store.NewJSONJobStore(filepath.Join(t.TempDir(), "state", "jobs.json"))
	if err != nil {
		t.Fatal(err)
	}
	service := application.NewService(gateway, jobs, filesystem.DownloadOrganizer{}, filesystem.JSONManifestStore{})
	handler := NewWithDownloadService(gateway, service)
	messages := make(chan protocol.Message, 20)
	handler.Handle(context.Background(), protocol.Request{
		Version: 1, ID: "app-job-1", Action: "start_chat_download",
		Params: json.RawMessage(fmt.Sprintf(`{"chat":{"id":123,"type":"channel","name":"Example"},"destinationRoot":%q,"filter":"videos_and_documents"}`, root)),
	}, func(m protocol.Message) { messages <- m })
	seenCompleted := false
	deadline := time.After(2 * time.Second)
	for !seenCompleted {
		select {
		case message := <-messages:
			if message.Event == "error" {
				t.Fatalf("start_chat_download error: %#v", message.Error)
			}
			seenCompleted = message.Event == "job_completed"
		case <-deadline:
			t.Fatal("timed out waiting for job_completed")
		}
	}
	wantFile := filepath.Join(root, "Example [123]", "Videos", "01. Caption Title.mp4")
	if _, err := os.Stat(wantFile); err != nil {
		t.Fatalf("expected organized caption filename %q: %v", wantFile, err)
	}
	var listed protocol.Message
	handler.Handle(context.Background(), protocol.Request{Version: 1, ID: "jobs", Action: "list_jobs"}, func(m protocol.Message) { listed = m })
	if listed.Event != "result" || !strings.Contains(mustJSON(t, listed.Data), `"state":"completed"`) {
		t.Fatalf("list_jobs result = %#v", listed)
	}
}

type pipelineTelegram struct {
	exportJSON string
	version    string
}

func (p *pipelineTelegram) Version(context.Context) (string, error) {
	if p.version != "" {
		return p.version, nil
	}
	return "Version: 0.20.4", nil
}
func (p *pipelineTelegram) SupportsGracefulPause() bool { return true }
func (p *pipelineTelegram) ManagedDownloadsSupported(version string) bool {
	return tdl.SupportsManagedDownloadVersion(version)
}
func (p *pipelineTelegram) SafeInterruptionSupported(version string) bool {
	return tdl.SupportsManagedDownloadVersion(version)
}
func (p *pipelineTelegram) MinimumSafeManagedDownloadVersion() string {
	return tdl.MinimumSafeManagedDownloadVersion
}
func (p *pipelineTelegram) ListChats(context.Context) ([]domain.Chat, error) {
	return []domain.Chat{{ID: 123, Type: "channel", Name: "Example"}}, nil
}
func (p *pipelineTelegram) ExportChat(_ context.Context, _ domain.Chat, outputPath string) error {
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o700); err != nil {
		return err
	}
	return os.WriteFile(outputPath, []byte(p.exportJSON), 0o600)
}
func (p *pipelineTelegram) Download(_ context.Context, request domain.DownloadRequest, onProgress func(domain.ProgressEvent)) error {
	data, err := os.ReadFile(request.ExportPath)
	if err != nil {
		return err
	}
	items, err := tdl.ParseExport(data)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(request.Destination, 0o700); err != nil {
		return err
	}
	for _, item := range items {
		name := fmt.Sprintf("%d_%d_%s", item.ChatID, item.MessageID, item.OriginalName)
		if err := os.WriteFile(filepath.Join(request.Destination, name), []byte("fixture video"), 0o600); err != nil {
			return err
		}
	}
	if onProgress != nil {
		percent := 100.0
		onProgress(domain.ProgressEvent{JobID: request.JobID, Percent: &percent})
	}
	return nil
}
func (p *pipelineTelegram) ParseExport(data []byte) ([]domain.MediaItem, error) {
	return tdl.ParseExport(data)
}
func (p *pipelineTelegram) FilterExport(data []byte, items []domain.MediaItem) ([]byte, error) {
	return tdl.FilterExport(data, items)
}

var _ ports.TelegramGateway = (*pipelineTelegram)(nil)
var _ ports.ExportProcessor = (*pipelineTelegram)(nil)

type blockingPipelineTelegram struct {
	*pipelineTelegram
}

func (p *blockingPipelineTelegram) Download(ctx context.Context, request domain.DownloadRequest, onProgress func(domain.ProgressEvent)) error {
	if onProgress != nil {
		percent := 15.0
		onProgress(domain.ProgressEvent{JobID: request.JobID, FileName: "pause-test.mp4", Percent: &percent})
	}
	<-ctx.Done()
	return ctx.Err()
}

func TestHandlerClosePausesActiveJobsWhenGracefulInterruptionIsSupported(t *testing.T) {
	root := t.TempDir()
	gateway := &blockingPipelineTelegram{pipelineTelegram: &pipelineTelegram{
		exportJSON: `{"id":123,"messages":[{"id":8,"type":"message","file":"pause-test.mp4","text":"Pause test"}]}`,
	}}
	jobs, err := store.NewJSONJobStore(filepath.Join(t.TempDir(), "state", "jobs.json"))
	if err != nil {
		t.Fatal(err)
	}
	service := application.NewService(gateway, jobs, filesystem.DownloadOrganizer{}, filesystem.JSONManifestStore{})
	handler := NewWithDownloadService(gateway, service)
	messages := make(chan protocol.Message, 30)
	handler.Handle(context.Background(), protocol.Request{
		Version: 1, ID: "graceful-close-job", Action: "start_chat_download",
		Params: json.RawMessage(fmt.Sprintf(`{"chat":{"id":123,"type":"channel","name":"Example"},"destinationRoot":%q,"filter":"videos_and_documents"}`, root)),
	}, func(m protocol.Message) { messages <- m })

	seenDownloading := false
	deadline := time.After(2 * time.Second)
	for !seenDownloading {
		select {
		case message := <-messages:
			if message.Event == "error" {
				t.Fatalf("start job error: %#v", message.Error)
			}
			if message.Event == "job_state" {
				payload := mustJSON(t, message.Data)
				seenDownloading = strings.Contains(payload, `"state":"downloading"`)
			}
		case <-deadline:
			t.Fatal("timed out waiting for downloading state")
		}
	}

	handler.Close()
	job, err := jobs.Get(context.Background(), "graceful-close-job")
	if err != nil {
		t.Fatal(err)
	}
	if job.State != domain.JobPaused {
		t.Fatalf("job state after graceful close = %q; want paused", job.State)
	}
}
