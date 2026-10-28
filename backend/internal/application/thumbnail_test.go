package application

import (
	"bytes"
	"context"
	"example.com/telegram-media-downloader/backend/internal/domain"
	"image"
	"image/jpeg"
	"testing"
	"time"
)

type previewGateway struct {
	*fakeGateway
	calls   int
	blocked bool
	started chan struct{}
}

func (g *previewGateway) StreamThumbnails(ctx context.Context, refs []domain.PreviewRequest, emit func(string, []byte)) error {
	g.calls++
	if g.blocked {
		close(g.started)
		<-ctx.Done()
		return ctx.Err()
	}

	for _, request := range refs {
		var b bytes.Buffer
		jpeg.Encode(&b, image.NewRGBA(image.Rect(0, 0, 320, 180)), nil)
		emit(request.Key, b.Bytes())
	}
	return nil
}
func thumbnailService(t *testing.T) (*Service, *previewGateway) {
	g := &previewGateway{fakeGateway: newFakeGateway(nil)}
	s, _ := newTestService(t, g.fakeGateway)
	s.Telegram = g
	s.PreviewDirectory = t.TempDir()
	s.snapshots = map[string]mediaSnapshot{"page": {Page: MediaPage{SnapshotID: "page", Items: []domain.MediaItem{{ChatID: 1, MessageID: 1, OriginalName: "a.mp4", Preview: &domain.ThumbnailReference{ID: 1, AccessHash: 2, DCID: 2, ThumbSize: "m", Size: 24000, FileReference: []byte{1}}}}}}}
	return s, g
}
func TestThumbnailUpgradeAndPersistentCache(t *testing.T) {
	s, g := thumbnailService(t)
	count := 0
	s.RequestThumbnails(context.Background(), "page", []string{"1:1:a.mp4"}, func(images map[string]string) { count += len(images) })
	if count != 1 || g.calls != 1 {
		t.Fatalf("count=%d calls=%d", count, g.calls)
	}
	s.RequestThumbnails(context.Background(), "page", []string{"1:1:a.mp4"}, func(images map[string]string) { count += len(images) })
	if count != 2 || g.calls != 1 {
		t.Fatal("thumbnail fetched again despite disk cache")
	}
}
func TestThumbnailWorkNeverCompetesWithAnActiveTransfer(t *testing.T) {
	s, g := thumbnailService(t)
	s.tdlSlot <- struct{}{}
	defer func() { <-s.tdlSlot }()
	s.RequestThumbnails(context.Background(), "page", []string{"1:1:a.mp4"}, func(map[string]string) { t.Fatal("unexpected image") })
	if g.calls != 0 {
		t.Fatal("thumbnail request ran while downloader was busy")
	}
}
func TestUnknownSnapshotCannotFetchThumbnail(t *testing.T) {
	s, g := thumbnailService(t)
	s.RequestThumbnails(context.Background(), "unknown", []string{"1:1:a.mp4"}, func(map[string]string) { t.Fatal("unexpected image") })
	if g.calls != 0 {
		t.Fatal("unknown snapshot invoked provider")
	}
}

func TestTransferPreemptsThumbnailWork(t *testing.T) {
	s, g := thumbnailService(t)
	g.blocked = true
	g.started = make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		s.RequestThumbnails(context.Background(), "page", []string{"1:1:a.mp4"}, func(map[string]string) { t.Error("cancelled preview published") })
	}()
	select {
	case <-g.started:
	case <-time.After(time.Second):
		t.Fatal("preview did not start")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	release, err := s.acquireTDL(ctx)
	if err != nil {
		t.Fatal("transfer could not preempt preview")
	}
	release()
	<-done
}
func TestCachedThumbnailRemainsAvailableDuringTransfers(t *testing.T) {
	s, g := thumbnailService(t)
	s.RequestThumbnails(context.Background(), "page", []string{"1:1:a.mp4"}, func(map[string]string) {})
	s.tdlSlot <- struct{}{}
	defer func() { <-s.tdlSlot }()
	count := 0
	s.RequestThumbnails(context.Background(), "page", []string{"1:1:a.mp4"}, func(images map[string]string) { count += len(images) })
	if count != 1 || g.calls != 1 {
		t.Fatal("cached thumbnail required network or was hidden")
	}
}
