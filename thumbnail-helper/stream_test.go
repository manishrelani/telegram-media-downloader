package main

import (
	"bytes"
	"context"
	"errors"
	"github.com/gotd/td/tg"
	"image"
	"image/jpeg"
	"sync"
	"testing"
	"time"
)

type blockingFiles struct {
	image []byte
	slow  <-chan struct{}
	fail  int64
}

func (f *blockingFiles) UploadGetFile(ctx context.Context, req *tg.UploadGetFileRequest) (tg.UploadFileClass, error) {
	id := req.Location.(*tg.InputDocumentFileLocation).ID
	if id == f.fail {
		return nil, errors.New("unavailable")
	}
	if id == 2 && f.slow != nil {
		select {
		case <-f.slow:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	return &tg.UploadFile{Bytes: f.image}, nil
}
func streamImage(t *testing.T) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := jpeg.Encode(&b, image.NewRGBA(image.Rect(0, 0, 320, 180)), nil); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}
func streamRequests() []Request {
	return []Request{{Key: "first", Reference: Reference{ID: 1, AccessHash: 2, FileReference: []byte{1}, DCID: 2, ThumbSize: "m", Size: 10000}}, {Key: "slow", Reference: Reference{ID: 2, AccessHash: 2, FileReference: []byte{1}, DCID: 2, ThumbSize: "m", Size: 10000}}}
}
func TestReadyImageStreamsBeforeBlockedSibling(t *testing.T) {
	slow := make(chan struct{})
	defer close(slow)
	api := &blockingFiles{image: streamImage(t), slow: slow}
	received := make(chan string, 2)
	done := make(chan error, 1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		done <- streamBatch(ctx, streamRequests(), func(context.Context, int) (fileAPI, error) { return api, nil }, func(key string, _ []byte) { received <- key })
	}()
	select {
	case key := <-received:
		if key != "first" {
			t.Fatalf("first result=%s", key)
		}
	case <-time.After(time.Second):
		t.Fatal("ready image waited for blocked sibling")
	}
	select {
	case <-done:
		t.Fatal("batch finished before sibling was released")
	default:
	}
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("cancellation did not stop batch")
	}
}
func TestFailedSiblingDoesNotDiscardSuccess(t *testing.T) {
	api := &blockingFiles{image: streamImage(t), fail: 2}
	got := []string{}
	err := streamBatch(context.Background(), streamRequests(), func(context.Context, int) (fileAPI, error) { return api, nil }, func(key string, _ []byte) { got = append(got, key) })
	if err != nil || len(got) != 1 || got[0] != "first" {
		t.Fatalf("results=%v err=%v", got, err)
	}
}
func TestDuplicateKeysAreRejectedBeforeNetwork(t *testing.T) {
	refs := streamRequests()
	refs[1].Key = "first"
	err := streamBatch(context.Background(), refs, func(context.Context, int) (fileAPI, error) {
		t.Error("invalid batch started network work")
		return nil, errors.New("bad")
	}, func(string, []byte) { t.Error("invalid batch emitted image") })
	if err == nil {
		t.Fatal("duplicate keys accepted")
	}
}
func TestStreamEmissionIsSerialized(t *testing.T) {
	api := &blockingFiles{image: streamImage(t)}
	var mu sync.Mutex
	count := 0
	err := streamBatch(context.Background(), streamRequests(), func(context.Context, int) (fileAPI, error) { return api, nil }, func(string, []byte) {
		if !mu.TryLock() {
			t.Error("concurrent callbacks")
			return
		}
		count++
		mu.Unlock()
	})
	if err != nil || count != 2 {
		t.Fatalf("count=%d err=%v", count, err)
	}
}
