package tdl

import (
	"bytes"
	"context"
	"encoding/json"
	"example.com/telegram-media-downloader/backend/internal/domain"
	"image"
	"image/jpeg"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type previewStreamExec struct {
	t       *testing.T
	release chan struct{}
	data    []byte
	key     string
}

func (e *previewStreamExec) Run(context.Context, ...string) ([]byte, []byte, error) {
	return []byte("tmm-previews-v3\n"), nil, nil
}
func (e *previewStreamExec) RunStream(ctx context.Context, args []string, emit func(string, string)) error {
	var input string
	for i, arg := range args {
		if arg == "--input" {
			input = args[i+1]
		}
	}
	raw, err := os.ReadFile(input)
	if err != nil {
		e.t.Error(err)
		return err
	}
	var requests []domain.PreviewRequest
	json.Unmarshal(raw, &requests)
	if e.key == "" && (len(requests) != 2 || requests[0].Key != "visible-b" || requests[1].Key != "visible-a") {
		e.t.Errorf("visible order was lost: %+v", requests)
	}
	line := func(protocol, key string, data []byte) string {
		encoded, _ := json.Marshal(map[string]any{"protocol": protocol, "key": key, "image": data})
		return string(encoded)
	}
	if e.key != "" {
		emit("stdout", line("tmm-previews-v3", e.key, e.data))
		return nil
	}
	emit("stdout", "not-json")
	emit("stderr", line("tmm-previews-v3", "visible-b", e.data))
	emit("stdout", line("other", "visible-b", e.data))
	emit("stdout", line("tmm-previews-v3", "offscreen", e.data))
	emit("stdout", line("tmm-previews-v3", "visible-b", []byte("broken")))
	emit("stdout", line("tmm-previews-v3", "visible-b", e.data))
	emit("stdout", line("tmm-previews-v3", "visible-b", e.data))
	select {
	case <-e.release:
	case <-ctx.Done():
		return ctx.Err()
	}
	return nil
}
func TestAdapterPublishesReadyImagesBeforeHelperExit(t *testing.T) {
	var picture bytes.Buffer
	jpeg.Encode(&picture, image.NewRGBA(image.Rect(0, 0, 320, 180)), nil)
	release := make(chan struct{})
	defer close(release)
	executor := &previewStreamExec{t: t, release: release, data: picture.Bytes()}
	helper := filepath.Join(t.TempDir(), "helper")
	os.WriteFile(helper, []byte("fixture"), 0700)
	adapter := DomainAdapter{Client: Client{Exec: executor}, ThumbnailHelper: helper}
	received := make(chan string, 8)
	done := make(chan error, 1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		done <- adapter.StreamThumbnails(ctx, []domain.PreviewRequest{{Key: "visible-b"}, {Key: "visible-a"}}, func(key string, _ []byte) { received <- key })
	}()
	select {
	case key := <-received:
		if key != "visible-b" {
			t.Fatalf("published %q", key)
		}
	case <-time.After(time.Second):
		t.Fatal("image was held until helper exit")
	}
	select {
	case key := <-received:
		t.Fatalf("unrequested or duplicate image %q", key)
	default:
	}
	cancel()
	<-done
}

func TestStreamAcceptsMaximumImageWithEscapedKey(t *testing.T) {
	var picture bytes.Buffer
	jpeg.Encode(&picture, image.NewRGBA(image.Rect(0, 0, 320, 180)), nil)
	data := append(picture.Bytes(), make([]byte, 256*1024-picture.Len())...)
	key := strings.Repeat("<", 2048)
	executor := &previewStreamExec{t: t, data: data, key: key}
	helper := filepath.Join(t.TempDir(), "helper")
	os.WriteFile(helper, []byte("fixture"), 0700)
	adapter := DomainAdapter{Client: Client{Exec: executor}, ThumbnailHelper: helper}
	count := 0
	err := adapter.StreamThumbnails(context.Background(), []domain.PreviewRequest{{Key: key}}, func(string, []byte) { count++ })
	if err != nil || count != 1 {
		t.Fatalf("valid bounded frame was rejected: count=%d err=%v", count, err)
	}
}
