package tdl

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"example.com/telegram-media-downloader/backend/internal/domain"
)

type fakeDomainExecutor struct {
	calls     [][]string
	stdout    []byte
	stderr    []byte
	err       error
	stream    []struct{ Stream, Text string }
	streamErr error
}

func (f *fakeDomainExecutor) Run(_ context.Context, args ...string) ([]byte, []byte, error) {
	f.calls = append(f.calls, append([]string(nil), args...))
	return f.stdout, f.stderr, f.err
}

func (f *fakeDomainExecutor) RunStream(_ context.Context, args []string, onOutput func(stream, text string)) error {
	f.calls = append(f.calls, append([]string(nil), args...))
	for _, item := range f.stream {
		onOutput(item.Stream, item.Text)
	}
	return f.streamErr
}

func TestDomainAdapterListChatsMapsToDomain(t *testing.T) {
	f := &fakeDomainExecutor{stdout: []byte(`[{"id":44,"type":"channel","visible_name":"GenAI","username":"genai"},{"id":55,"type":"private","visible_name":"Alex"}]`)}
	got, err := (DomainAdapter{Client: Client{Exec: f}}).ListChats(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	want := []domain.Chat{{ID: 44, Type: "channel", Name: "GenAI", Username: "genai"}, {ID: 55, Type: "private", Name: "Alex"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ListChats() = %#v, want %#v", got, want)
	}
}

func TestDomainAdapterExportUsesUsernameWhenAvailable(t *testing.T) {
	f := &fakeDomainExecutor{}
	chat := domain.Chat{ID: 44, Type: "channel", Name: "GenAI", Username: "genai"}
	if err := (DomainAdapter{Client: Client{Exec: f}}).ExportChat(context.Background(), chat, "/tmp/export with spaces.json"); err != nil {
		t.Fatal(err)
	}
	want := [][]string{{"chat", "export", "-c", "@genai", "--with-content", "--raw", "-o", "/tmp/export with spaces.json"}}
	if !reflect.DeepEqual(f.calls, want) {
		t.Fatalf("calls = %#v, want %#v", f.calls, want)
	}
}

func TestDomainAdapterExportFallsBackToChatID(t *testing.T) {
	f := &fakeDomainExecutor{}
	chat := domain.Chat{ID: -100123, Type: "channel", Name: "Private Channel"}
	if err := (DomainAdapter{Client: Client{Exec: f}}).ExportChat(context.Background(), chat, "/tmp/export.json"); err != nil {
		t.Fatal(err)
	}
	if f.calls[0][3] != "-100123" {
		t.Fatalf("chat selector = %q, want ID fallback", f.calls[0][3])
	}
}

func TestDomainAdapterDownloadMapsFilterAndProgress(t *testing.T) {
	f := &fakeDomainExecutor{stream: []struct{ Stream, Text string }{
		{Stream: "stdout", Text: "Lecture 35.1% [98.00 MB in 49.682s; ~ETA: 1m32s; 1.97 MB/s"},
		{Stream: "stderr", Text: "CPU: 0.31% Memory: 41.89 MB Goroutines: 84"},
	}}
	var events []domain.ProgressEvent
	req := domain.DownloadRequest{JobID: "job-1", Chat: domain.Chat{ID: 44}, ExportPath: "/tmp/export.json", Destination: "/Volumes/Data/GenAI", Filter: domain.FilterVideosAndDocuments, Takeout: true, Continue: true}
	err := (DomainAdapter{Client: Client{Exec: f}}).Download(context.Background(), req, func(event domain.ProgressEvent) { events = append(events, event) })
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatalf("progress events = %#v, want exactly one recognized stdout progress event", events)
	}
	if events[0].JobID != "job-1" || events[0].Percent == nil || *events[0].Percent < 35.09 || *events[0].Percent > 35.11 {
		t.Fatalf("progress event = %+v", events[0])
	}
	if events[0].FileName != "Lecture" {
		t.Fatalf("current file label = %q, want parsed progress label", events[0].FileName)
	}
	if events[0].SpeedBytesPerSecond == nil || *events[0].SpeedBytesPerSecond <= 0 {
		t.Fatalf("speed was not parsed: %+v", events[0])
	}
	if events[0].ETASeconds == nil || *events[0].ETASeconds != 92 {
		t.Fatalf("ETA = %+v, want 92 seconds", events[0].ETASeconds)
	}
	if len(f.calls) != 1 {
		t.Fatalf("expected one command call, got %#v", f.calls)
	}
	joined := strings.Join(f.calls[0], " ")
	if !strings.Contains(joined, "-i") || !strings.Contains(joined, "--takeout") || !strings.Contains(joined, "--continue") {
		t.Fatalf("download args do not contain filter/takeout/continue: %#v", f.calls[0])
	}
}

func TestDomainAdapterAllAttachmentsDoesNotAddExtensionWhitelist(t *testing.T) {
	f := &fakeDomainExecutor{}
	req := domain.DownloadRequest{JobID: "all", ExportPath: "/tmp/export.json", Destination: "/tmp/out", Filter: domain.FilterAllAttachments}
	if err := (DomainAdapter{Client: Client{Exec: f}}).Download(context.Background(), req, nil); err != nil {
		t.Fatal(err)
	}
	for _, arg := range f.calls[0] {
		if arg == "-i" {
			t.Fatalf("all-attachments mode should not add extension filter: %#v", f.calls[0])
		}
	}
}

func TestDomainAdapterParsesProgressFromStderrWhenTdlWritesItThere(t *testing.T) {
	f := &fakeDomainExecutor{stream: []struct{ Stream, Text string }{
		{Stream: "stderr", Text: "Namaste Node.js 35.1% [98.00 MB in 49.682s; ~ETA: 1m32s; 1.97 MB/s"},
		{Stream: "stderr", Text: "CPU: 0.31% Memory: 41.89 MB Goroutines: 84"},
	}}
	var events []domain.ProgressEvent
	err := (DomainAdapter{Client: Client{Exec: f}}).Download(context.Background(), domain.DownloadRequest{
		JobID: "stderr-progress", ExportPath: "/tmp/export.json", Destination: "/tmp/out", Filter: domain.FilterVideosAndDocuments,
	}, func(event domain.ProgressEvent) { events = append(events, event) })
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatalf("events = %#v; want one progress event", events)
	}
	if events[0].Percent == nil || *events[0].Percent < 35.09 || *events[0].Percent > 35.11 {
		t.Fatalf("percent = %#v; want about 35.1", events[0].Percent)
	}
}
