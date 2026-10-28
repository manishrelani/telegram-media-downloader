package application

import (
	"context"
	"example.com/telegram-media-downloader/backend/internal/domain"
	"sync"
	"testing"
	"time"
)

type firstThenWait struct {
	*previewGateway
	release chan struct{}
}

func (g *firstThenWait) StreamThumbnails(ctx context.Context, requests []domain.PreviewRequest, emit func(string, []byte)) error {
	g.previewGateway.StreamThumbnails(ctx, requests[:1], emit)
	select {
	case <-g.release:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
func TestApplicationPublishesFirstImageBeforeProducerFinishes(t *testing.T) {
	s, g := thumbnailService(t)
	release := make(chan struct{})
	defer close(release)
	s.Telegram = &firstThenWait{previewGateway: g, release: release}
	received := make(chan struct{}, 1)
	done := make(chan struct{})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		defer close(done)
		s.RequestThumbnails(ctx, "page", []string{"1:1:a.mp4"}, func(images map[string]string) {
			if len(images) == 1 {
				received <- struct{}{}
			}
		})
	}()
	select {
	case <-received:
	case <-time.After(time.Second):
		t.Fatal("ready image held until producer completion")
	}
	select {
	case <-done:
		t.Fatal("producer should still be blocked")
	default:
	}
	cancel()
	<-done
}
func TestVisibleRequestsRejectUnknownKeysBeforePublishing(t *testing.T) {
	s, g := thumbnailService(t)
	_, err := s.RequestThumbnails(context.Background(), "page", []string{"not-in-snapshot"}, func(map[string]string) { t.Error("unrequested image published") })
	if err == nil || g.calls != 0 {
		t.Fatal("snapshot membership was not enforced")
	}
}
func TestVisibleRequestDoesNotFetchUnrequestedPageVideos(t *testing.T) {
	s, g := thumbnailService(t)
	page := s.snapshots["page"]
	page.Page.Items = append(page.Page.Items, domain.MediaItem{ChatID: 1, MessageID: 2, OriginalName: "offscreen.mp4", Preview: page.Page.Items[0].Preview})
	s.snapshots["page"] = page
	got := []string{}
	s.RequestThumbnails(context.Background(), "page", []string{"1:1:a.mp4", "1:1:a.mp4"}, func(images map[string]string) {
		for key := range images {
			got = append(got, key)
		}
	})
	if len(got) != 1 || got[0] != "1:1:a.mp4" || g.calls != 1 {
		t.Fatalf("visible results=%v", got)
	}
}

func TestPreemptedVisibleRequestCanRetryAfterDownload(t *testing.T) {
	s, g := thumbnailService(t)
	g.blocked = true
	g.started = make(chan struct{})
	done := make(chan struct {
		busy bool
		err  error
	}, 1)
	go func() {
		busy, err := s.RequestThumbnails(context.Background(), "page", []string{"1:1:a.mp4"}, func(map[string]string) {})
		done <- struct {
			busy bool
			err  error
		}{busy, err}
	}()
	select {
	case <-g.started:
	case <-time.After(time.Second):
		t.Fatal("preview did not start")
	}
	s.cancelPreviews()
	select {
	case result := <-done:
		if !result.busy || result.err != nil {
			t.Fatalf("preempted preview must retry: %+v", result)
		}
	case <-time.After(time.Second):
		t.Fatal("preview did not stop")
	}
}

// Deadline is the context-construction boundary. Holding previewMu lets this
// test pause acquisition before cancellation can be registered.
type previewDeadlineGate struct {
	context.Context
	reached chan struct{}
	once    sync.Once
}

func (g *previewDeadlineGate) Deadline() (time.Time, bool) {
	g.once.Do(func() { close(g.reached) })
	return g.Context.Deadline()
}
func TestPreviewCannotClaimSlotBeforeCancellationRegistration(t *testing.T) {
	s, _ := thumbnailService(t)
	parent, cancel := context.WithCancel(context.Background())
	defer cancel()
	gate := &previewDeadlineGate{Context: parent, reached: make(chan struct{})}
	s.previewMu.Lock()
	unlocked := false
	defer func() {
		if !unlocked {
			s.previewMu.Unlock()
		}
	}()
	done := make(chan struct{})
	go func() {
		defer close(done)
		_, release, ok := s.tryAcquirePreview(gate)
		if ok {
			release()
		}
	}()
	select {
	case <-gate.reached:
	case <-time.After(time.Second):
		t.Fatal("preview did not reach context boundary")
	}
	if len(s.tdlSlot) != 0 {
		t.Error("preview claimed CLI slot before it could register cancellation")
	}
	s.previewMu.Unlock()
	unlocked = true
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("preview did not release")
	}
}

type transferSelectGate struct {
	context.Context
	reached, proceed chan struct{}
	once             sync.Once
}

func (g *transferSelectGate) Done() <-chan struct{} {
	g.once.Do(func() { close(g.reached); <-g.proceed })
	return g.Context.Done()
}
func TestWaitingTransferPreventsNewPreviewFromClaimingFreeSlot(t *testing.T) {
	s, _ := thumbnailService(t)
	gate := &transferSelectGate{Context: context.Background(), reached: make(chan struct{}), proceed: make(chan struct{})}
	acquired := make(chan func(), 1)
	go func() { release, _ := s.acquireTDL(gate); acquired <- release }()
	select {
	case <-gate.reached:
	case <-time.After(time.Second):
		t.Fatal("transfer did not reach handoff")
	}
	_, release, ok := s.tryAcquirePreview(context.Background())
	if ok {
		release()
		t.Error("new preview bypassed a waiting transfer")
	}
	close(gate.proceed)
	select {
	case release := <-acquired:
		release()
	case <-time.After(time.Second):
		t.Fatal("transfer did not acquire CLI")
	}
}
