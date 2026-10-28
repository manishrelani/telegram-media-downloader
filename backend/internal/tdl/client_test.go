package tdl

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"
)

type fakeExecutor struct {
	calls  [][]string
	stdout []byte
	stderr []byte
	err    error
}

func (f *fakeExecutor) Run(_ context.Context, args ...string) ([]byte, []byte, error) {
	f.calls = append(f.calls, append([]string(nil), args...))
	return f.stdout, f.stderr, f.err
}

func TestListChatsBuildsDocumentedCommandAndParsesJSON(t *testing.T) {
	f := &fakeExecutor{stdout: []byte(`[{"id":4488394154,"type":"channel","visible_name":"GenAI Course","username":"chaicode"},{"id":22,"type":"private","visible_name":"Alex"}]`)}
	got, err := (Client{Exec: f}).ListChats(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].ID != 4488394154 || got[0].Type != "channel" || got[1].Type != "private" {
		t.Fatalf("unexpected chats: %#v", got)
	}
	want := [][]string{{"chat", "ls", "-o", "json"}}
	if !reflect.DeepEqual(f.calls, want) {
		t.Fatalf("calls = %#v, want %#v", f.calls, want)
	}
}

func TestListChatsRejectsInvalidJSON(t *testing.T) {
	f := &fakeExecutor{stdout: []byte("not json")}
	_, err := (Client{Exec: f}).ListChats(context.Background())
	if err == nil || !strings.Contains(err.Error(), "parse tdl chat list JSON") {
		t.Fatalf("expected parse error, got %v", err)
	}
}

func TestExportChatIncludesRawIdentityAndSafeArgumentBoundaries(t *testing.T) {
	f := &fakeExecutor{}
	err := (Client{Exec: f}).ExportChat(context.Background(), "@example channel", "/tmp/export with spaces.json")
	if err != nil {
		t.Fatal(err)
	}
	want := [][]string{{"chat", "export", "-c", "@example channel", "--with-content", "--raw", "-o", "/tmp/export with spaces.json"}}
	if !reflect.DeepEqual(f.calls, want) {
		t.Fatalf("calls = %#v, want %#v", f.calls, want)
	}
}

func TestBuildDownloadArgsIncludesSelectedOptions(t *testing.T) {
	got, err := BuildDownloadArgs(DownloadOptions{
		ExportPath: "/tmp/export.json", Directory: "/Volumes/Data/Downloads",
		Include: []string{"mp4", "pdf"}, Takeout: true, Continue: true, Group: true, SkipSame: true,
		Template: "{{ .DialogID }}_{{ .MessageID }}_{{ filenamify .FileName }}",
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"dl", "-f", "/tmp/export.json", "-d", "/Volumes/Data/Downloads", "-i", "mp4,pdf", "--template", "{{ .DialogID }}_{{ .MessageID }}_{{ filenamify .FileName }}", "--takeout", "--continue", "--group", "--skip-same"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("args = %#v, want %#v", got, want)
	}
}

func TestBuildDownloadArgsRequiresPaths(t *testing.T) {
	for _, o := range []DownloadOptions{{Directory: "/tmp"}, {ExportPath: "/tmp/export.json"}} {
		if _, err := BuildDownloadArgs(o); err == nil {
			t.Fatalf("expected validation error for %+v", o)
		}
	}
}

func TestCommandErrorPreservesHelpfulStderr(t *testing.T) {
	got := commandError("tdl chat ls", []byte("Current database is used by another process"), errors.New("exit status 1"))
	if !strings.Contains(got.Error(), "database is used by another process") {
		t.Fatalf("missing stderr detail: %v", got)
	}
}

func TestReadSegmentsHandlesCarriageReturnAndNewline(t *testing.T) {
	input := "first update\rsecond update\nthird update\r\nfinal partial"
	var got []string
	if err := readSegments(strings.NewReader(input), func(s string) { got = append(got, s) }); err != nil {
		t.Fatal(err)
	}
	want := []string{"first update", "second update", "third update", "final partial"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("segments = %#v, want %#v", got, want)
	}
}

func TestOSExecutorStreamsCarriageReturnProgress(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fixture uses a POSIX shell; Windows process behavior is a separate validation task")
	}
	sh, err := exec.LookPath("sh")
	if err != nil {
		t.Skip("sh is not available")
	}
	fixture := filepath.Join(t.TempDir(), "fake-tdl.sh")
	script := "#!/bin/sh\nprintf 'Video 35.1%% [98.00 MB in 49.682s; ~ETA: 1m32s; 1.97 MB/s\\r'\nprintf 'Video 42.0%% [110.00 MB in 55.0s; ~ETA: 1m10s; 2.00 MB/s\\r'\nprintf 'diagnostic warning\\n' >&2\n"
	if err := os.WriteFile(fixture, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	// Execute through sh for predictable fixture behavior; arguments are not interpreted by a shell by production code.
	var lines []string
	var streams []string
	if err := (OSExecutor{Binary: sh}).RunStream(context.Background(), []string{fixture}, func(stream, line string) { streams = append(streams, stream); lines = append(lines, line) }); err != nil {
		t.Fatal(err)
	}
	count := 0
	for i, line := range lines {
		if streams[i] == "stdout" {
			if _, ok := progressLine(line); ok {
				count++
			}
		}
	}
	if count != 2 {
		t.Fatalf("parsed stream lines = %#v; expected two stdout progress updates", lines)
	}
	foundStderr := false
	for i, line := range lines {
		if streams[i] == "stderr" && line == "diagnostic warning" {
			foundStderr = true
		}
	}
	if !foundStderr {
		t.Fatalf("stderr was not streamed: %#v", lines)
	}
}

func progressLine(line string) (string, bool) {
	if strings.Contains(line, "%") && strings.Contains(line, "ETA:") {
		return line, true
	}
	return "", false
}

func TestOSExecutorContextCancellationSendsGracefulInterrupt(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows graceful pause is not supported yet")
	}
	sh, err := exec.LookPath("sh")
	if err != nil {
		t.Skip("sh is not available")
	}
	dir := t.TempDir()
	fixture := filepath.Join(dir, "interruptible.sh")
	started := filepath.Join(dir, "started")
	interrupted := filepath.Join(dir, "interrupted")
	script := "#!/bin/sh\ntrap 'echo yes > \"" + interrupted + "\"; exit 23' INT\necho started > \"" + started + "\"\nwhile :; do printf 'Downloading 10%% [1.00 MB in 1s; ~ETA: 1m; 1.00 MB/s\\r'; sleep 0.05; done\n"
	if err := os.WriteFile(fixture, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		done <- (OSExecutor{Binary: sh}).RunStream(ctx, []string{fixture}, nil)
	}()
	deadline := time.Now().Add(2 * time.Second)
	for {
		if _, err := os.Stat(started); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("fixture process did not start")
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("interrupted command unexpectedly succeeded")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("RunStream did not exit after cancellation")
	}
	if _, err := os.Stat(interrupted); err != nil {
		t.Fatalf("child did not run its graceful interrupt handler: %v", err)
	}
}

func TestOSExecutorRunContextCancellationSendsGracefulInterrupt(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows graceful pause is not supported yet")
	}
	sh, err := exec.LookPath("sh")
	if err != nil {
		t.Skip("sh is not available")
	}
	dir := t.TempDir()
	fixture := filepath.Join(dir, "interruptible-run.sh")
	started := filepath.Join(dir, "started")
	interrupted := filepath.Join(dir, "interrupted")
	script := "#!/bin/sh\ntrap 'echo yes > \"" + interrupted + "\"; exit 23' INT\necho started > \"" + started + "\"\nwhile :; do sleep 0.05; done\n"
	if err := os.WriteFile(fixture, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, _, runErr := (OSExecutor{Binary: sh}).Run(ctx, fixture)
		done <- runErr
	}()
	deadline := time.Now().Add(2 * time.Second)
	for {
		if _, err := os.Stat(started); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("fixture process did not start")
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("interrupted command unexpectedly succeeded")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Run did not exit after cancellation")
	}
	if _, err := os.Stat(interrupted); err != nil {
		t.Fatalf("child did not run its graceful interrupt handler: %v", err)
	}
}
