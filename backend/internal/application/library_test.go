package application

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"example.com/telegram-media-downloader/backend/internal/domain"
)

type libraryGateway struct {
	*fakeGateway
	query    domain.MediaQuery
	omitFile bool
}

func TestLibraryPauseRestartResumePreservesCompletedFilesAndScope(t *testing.T) {
	fake := newFakeGateway([]fakeMedia{{id: 3, name: "one.mp4"}, {id: 2, name: "two.pdf"}})
	fake.waitForCancelOnCall = 2
	s, jobs := newTestService(t, fake)
	gateway := &libraryGateway{fakeGateway: fake}
	s.Telegram = gateway
	chat := domain.Chat{ID: 123, Name: "Course"}
	topic := &domain.Topic{ID: 42, Title: "Backend"}
	page, err := s.ListMedia(context.Background(), domain.MediaQuery{Chat: chat, Topic: topic, Limit: 100})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancelCause(context.Background())
	done := make(chan RunResult, 1)
	go func() {
		result, err := s.RunMedia(ctx, MediaRunRequest{JobID: "pause-library", Chat: chat, DestinationRoot: t.TempDir(), Selection: domain.MediaSelection{Topic: topic, FileTypes: []string{"video", "pdf"}, SnapshotID: page.SnapshotID, Keys: []string{"123:3:one.mp4", "123:2:two.pdf"}}}, nil)
		if err != nil {
			t.Errorf("run: %v", err)
		}
		done <- result
	}()
	select {
	case <-fake.downloadStarted:
	case <-time.After(3 * time.Second):
		t.Fatal("second file did not start")
	}
	cancel(ErrPauseRequested)
	var paused RunResult
	select {
	case paused = <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("pause did not finish")
	}
	if paused.Job.State != domain.JobPaused || paused.Job.Files[0].State != "completed" || paused.Job.Files[1].State != "paused" {
		t.Fatalf("paused=%+v", paused.Job)
	}
	// The fresh service has no catalog cache: continuation must use the saved export.
	fresh := NewService(gateway, jobs, s.Organizer, s.Manifests)
	result, err := fresh.Resume(context.Background(), "pause-library", nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.Job.State != domain.JobCompleted || fake.downloadCalls != 3 || fake.exportCalls != 1 || !fake.lastContinue {
		t.Fatalf("resume=%+v calls=%d exports=%d continue=%v", result, fake.downloadCalls, fake.exportCalls, fake.lastContinue)
	}
}

func TestLibraryRetryDoesNotRedownloadCompletedFiles(t *testing.T) {
	fake := newFakeGateway([]fakeMedia{{id: 2, name: "ok.pdf"}, {id: 1, name: "retry.pdf"}})
	fake.downloadErrors = []error{nil, fmt.Errorf("transfer failed")}
	s, _ := newTestService(t, fake)
	s.Telegram = &libraryGateway{fakeGateway: fake}
	chat := domain.Chat{ID: 123, Name: "Course"}
	result, err := s.RunMedia(context.Background(), MediaRunRequest{JobID: "retry-library", Chat: chat, DestinationRoot: t.TempDir(), Selection: domain.MediaSelection{FileTypes: []string{"pdf"}, EntireScope: true}}, nil)
	if err == nil || result.Job.Files[0].State != "completed" || result.Job.Files[1].State != "failed" {
		t.Fatalf("first=%+v %v", result, err)
	}
	result, err = s.RetryMedia(context.Background(), "retry-library", nil)
	if err != nil || result.Job.State != domain.JobCompleted || fake.downloadCalls != 3 || fake.exportCalls != 1 {
		t.Fatalf("retry=%+v %v", result, err)
	}
}

func TestLibraryRecoveryNormalizesFileStatesAndReconstructsDiscovery(t *testing.T) {
	fake := newFakeGateway([]fakeMedia{{id: 1, name: "notes.pdf"}})
	s, jobs := newTestService(t, fake)
	s.Telegram = &libraryGateway{fakeGateway: fake}
	root := t.TempDir()
	job := domain.DownloadJob{ID: "crash", Library: true, Chat: domain.Chat{ID: 123, Name: "Course"}, State: domain.JobDownloading, Destination: filepath.Join(root, "Course [123]"), DestinationRoot: root, Filter: domain.FilterAllAttachments, FileTypes: []string{"pdf"}, Selection: &domain.MediaSelection{FileTypes: []string{"pdf"}, EntireScope: true}}
	if err := os.MkdirAll(mediaWorkDir(job), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := fake.ExportChat(context.Background(), job.Chat, filepath.Join(mediaWorkDir(job), "export.json")); err != nil {
		t.Fatal(err)
	}
	if err := jobs.Save(context.Background(), job); err != nil {
		t.Fatal(err)
	}
	if _, err := s.RecoverInterruptedJobs(context.Background()); err != nil {
		t.Fatal(err)
	}
	result, err := s.Resume(context.Background(), job.ID, nil)
	if err != nil || result.Job.WorkDirectory == "" || len(result.Job.Files) != 1 || result.Job.Files[0].State != "completed" {
		t.Fatalf("recovered=%+v %v", result, err)
	}
	job.ID = "file-crash"
	job.Files = []domain.FileTask{{Item: domain.MediaItem{ChatID: 123, MessageID: 2, OriginalName: "a.pdf"}, State: "downloading"}}
	if err := jobs.Save(context.Background(), job); err != nil {
		t.Fatal(err)
	}
	if _, err := s.RecoverInterruptedJobs(context.Background()); err != nil {
		t.Fatal(err)
	}
	job, _ = jobs.Get(context.Background(), job.ID)
	if job.Files[0].State != "paused" {
		t.Fatal("recovered row still marked downloading")
	}
	job, err = s.CancelPersistedJob(context.Background(), job.ID, nil)
	if err != nil || job.Files[0].State != "cancelled" {
		t.Fatalf("cancelled=%+v %v", job, err)
	}
}

func TestLibraryRefreshUsesNewestSnapshotAndFiltersWholeScope(t *testing.T) {
	fake := newFakeGateway([]fakeMedia{{id: 1, name: "first.pdf"}, {id: 2, name: "video.mp4"}})
	s, _ := newTestService(t, fake)
	s.Telegram = &libraryGateway{fakeGateway: fake}
	query := domain.MediaQuery{Chat: domain.Chat{ID: 123, Name: "Course"}, Topic: &domain.Topic{ID: 42, Title: "Backend"}, Limit: 100}
	first, err := s.ListMedia(context.Background(), query)
	if err != nil {
		t.Fatal(err)
	}
	fake.items = append(fake.items, fakeMedia{id: 3, name: "new.pdf"})
	query.Refresh = true
	latest, err := s.ListMedia(context.Background(), query)
	if err != nil {
		t.Fatal(err)
	}
	query.Refresh = false
	cached, err := s.ListMedia(context.Background(), query)
	if err != nil || cached.SnapshotID != latest.SnapshotID || cached.SnapshotID == first.SnapshotID || !cached.Cached {
		t.Fatalf("cache=%+v %v", cached, err)
	}
	result, err := s.RunMedia(context.Background(), MediaRunRequest{JobID: "entire", Chat: query.Chat, DestinationRoot: t.TempDir(), Selection: domain.MediaSelection{Topic: query.Topic, FileTypes: []string{"pdf"}, EntireScope: true}}, nil)
	if err != nil || len(result.Job.Files) != 2 {
		t.Fatalf("scope=%+v %v", result, err)
	}
}

func (g *libraryGateway) ExportMedia(ctx context.Context, query domain.MediaQuery, output string) error {
	g.query = query
	return g.ExportChat(ctx, query.Chat, output)
}
func (g *libraryGateway) Download(ctx context.Context, req domain.DownloadRequest, progress func(domain.ProgressEvent)) error {
	if g.omitFile {
		return nil
	}
	return g.fakeGateway.Download(ctx, req, progress)
}

func TestLibrarySelectionIsExactAndProgressHasStableIdentity(t *testing.T) {
	fake := newFakeGateway([]fakeMedia{{id: 3, name: "lesson.mp4"}, {id: 2, name: "notes.pdf"}, {id: 1, name: "secret.pdf"}})
	s, jobs := newTestService(t, fake)
	gateway := &libraryGateway{fakeGateway: fake}
	s.Telegram = gateway
	chat := domain.Chat{ID: 123, Name: "Course"}
	topic := &domain.Topic{ID: 42, Title: "Backend"}
	page, err := s.ListMedia(context.Background(), domain.MediaQuery{Chat: chat, Topic: topic, Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if !page.HasMore || len(page.Items) != 2 {
		t.Fatalf("page=%+v", page)
	}
	var progressCount int
	result, err := s.RunMedia(context.Background(), MediaRunRequest{JobID: "selected", Chat: chat, DestinationRoot: t.TempDir(), Selection: domain.MediaSelection{Topic: topic, FileTypes: []string{"pdf"}, SnapshotID: page.SnapshotID, Keys: []string{"123:2:notes.pdf"}}}, func(e Event) {
		if e.Progress != nil {
			if e.Progress.ItemKey != "123:2:notes.pdf" {
				t.Errorf("progress=%+v", e.Progress)
			}
			progressCount++
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Job.Files) != 1 || result.Job.Files[0].State != "completed" || progressCount == 0 {
		t.Fatalf("result=%+v", result)
	}
	if fake.downloadCalls != 1 || fake.lastDownloadedItems != 1 {
		t.Fatal("download expanded beyond exact selection")
	}
	if _, err := os.Stat(filepath.Join(result.Job.Destination, "Backend [42]", "Documents", "notes.pdf")); err != nil {
		t.Fatal(err)
	}
	persisted, _ := jobs.Get(context.Background(), "selected")
	if persisted.Files[0].SavedPath == "" {
		t.Fatal("missing saved file path")
	}
}

func TestLibraryRejectsOutOfScopeKeysAndMissingOutputs(t *testing.T) {
	fake := newFakeGateway([]fakeMedia{{id: 1, name: "notes.pdf"}})
	s, _ := newTestService(t, fake)
	gateway := &libraryGateway{fakeGateway: fake, omitFile: true}
	s.Telegram = gateway
	chat := domain.Chat{ID: 123, Name: "Course"}
	page, err := s.ListMedia(context.Background(), domain.MediaQuery{Chat: chat, Limit: 100})
	if err != nil {
		t.Fatal(err)
	}
	request := MediaRunRequest{JobID: "invalid", Chat: chat, DestinationRoot: t.TempDir(), Selection: domain.MediaSelection{FileTypes: []string{"pdf"}, SnapshotID: page.SnapshotID, Keys: []string{"123:2:other.pdf"}}}
	if _, err := s.RunMedia(context.Background(), request, nil); err == nil {
		t.Fatal("accepted unseen file")
	}
	request.JobID = "missing"
	request.Selection.Keys = []string{"123:1:notes.pdf"}
	result, err := s.RunMedia(context.Background(), request, nil)
	if err == nil || result.Job.State != domain.JobFailed || result.Job.Files[0].State != "failed" {
		t.Fatalf("missing output must fail: %+v %v", result, err)
	}
}
