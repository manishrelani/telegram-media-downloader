package application

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"example.com/telegram-media-downloader/backend/internal/domain"
	"example.com/telegram-media-downloader/backend/internal/filesystem"
	"example.com/telegram-media-downloader/backend/internal/ports"
	"example.com/telegram-media-downloader/backend/internal/store"
	"example.com/telegram-media-downloader/backend/internal/tdl"
)

func TestServiceRunExportsDownloadsOrganizesAndPersistsJob(t *testing.T) {
	root := t.TempDir()
	gateway := newFakeGateway([]fakeMedia{
		{id: 26, name: "lesson.mp4", caption: "01. Introduction to GenAI"},
		{id: 25, name: "notes.pdf", caption: "RAG Architecture Notes"},
	})
	service, jobs := newTestService(t, gateway)
	var events []Event
	result, err := service.Run(context.Background(), RunRequest{
		JobID: "job-1", Chat: domain.Chat{ID: 123, Type: "channel", Name: "GenAI Course"},
		DestinationRoot: root, Filter: domain.FilterVideosAndDocuments, Takeout: true,
	}, func(event Event) { events = append(events, event) })
	if err != nil {
		t.Fatal(err)
	}
	if result.Job.State != domain.JobCompleted {
		t.Fatalf("final state = %s, want completed", result.Job.State)
	}
	if result.Summary.Renamed != 2 || result.Summary.Failed != 0 {
		t.Fatalf("organization summary = %#v", result.Summary)
	}
	chatRoot := filepath.Join(root, "GenAI Course [123]")
	for _, file := range []string{
		filepath.Join(chatRoot, "Videos", "01. Introduction to GenAI.mp4"),
		filepath.Join(chatRoot, "Documents", "RAG Architecture Notes.pdf"),
	} {
		if _, err := os.Stat(file); err != nil {
			t.Errorf("expected downloaded file %q: %v", file, err)
		}
	}
	persisted, err := jobs.Get(context.Background(), "job-1")
	if err != nil {
		t.Fatal(err)
	}
	if persisted.State != domain.JobCompleted || len(persisted.Items) != 2 {
		t.Fatalf("persisted job = %#v", persisted)
	}
	if gateway.downloadCalls != 1 {
		t.Fatalf("download calls = %d, want 1", gateway.downloadCalls)
	}
	if !hasEvent(events, "job_state") || !hasEvent(events, "download_progress") || !hasEvent(events, "job_completed") {
		t.Fatalf("missing expected events: %#v", events)
	}
}

func TestServiceRunUsesCaptionNamesAndDoesNotOverwriteDuplicateTitles(t *testing.T) {
	root := t.TempDir()
	gateway := newFakeGateway([]fakeMedia{
		{id: 26, name: "first.mp4", caption: "Same Lecture"},
		{id: 25, name: "second.mp4", caption: "Same Lecture"},
	})
	service, _ := newTestService(t, gateway)
	result, err := service.Run(context.Background(), RunRequest{
		JobID: "job-duplicate", Chat: domain.Chat{ID: 456, Type: "channel", Name: "Lessons"},
		DestinationRoot: root, Filter: domain.FilterVideosAndDocuments,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.Summary.Renamed != 2 {
		t.Fatalf("summary = %#v", result.Summary)
	}
	for _, name := range []string{"Same Lecture.mp4", "Same Lecture (2).mp4"} {
		if _, err := os.Stat(filepath.Join(root, "Lessons [456]", "Videos", name)); err != nil {
			t.Errorf("expected non-overwriting output %q: %v", name, err)
		}
	}
}

func TestServiceUsesChatIDToSeparateIdenticallyNamedChats(t *testing.T) {
	root := t.TempDir()
	gateway := newFakeGateway([]fakeMedia{{id: 7, name: "lesson.mp4", caption: "Same Lesson"}})
	service, _ := newTestService(t, gateway)

	for _, chatID := range []int64{901, 902} {
		_, err := service.Run(context.Background(), RunRequest{
			JobID:           fmt.Sprintf("same-name-%d", chatID),
			Chat:            domain.Chat{ID: chatID, Type: "channel", Name: "Course"},
			DestinationRoot: root, Filter: domain.FilterVideosAndDocuments,
		}, nil)
		if err != nil {
			t.Fatalf("Run() for chat %d: %v", chatID, err)
		}
	}

	for _, chatID := range []int64{901, 902} {
		path := filepath.Join(root, fmt.Sprintf("Course [%d]", chatID), "Videos", "Same Lesson.mp4")
		if _, err := os.Stat(path); err != nil {
			t.Errorf("expected independent output for chat %d at %q: %v", chatID, path, err)
		}
	}
}

func TestServiceRunSkipsItemsAlreadyCompletedInManifest(t *testing.T) {
	root := t.TempDir()
	gateway := newFakeGateway([]fakeMedia{{id: 7, name: "lesson.mp4", caption: "Lesson 1"}})
	service, _ := newTestService(t, gateway)
	request := RunRequest{Chat: domain.Chat{ID: 789, Type: "channel", Name: "Replay"}, DestinationRoot: root, Filter: domain.FilterVideosAndDocuments, SkipSame: true}
	request.JobID = "first"
	if _, err := service.Run(context.Background(), request, nil); err != nil {
		t.Fatal(err)
	}
	calls := gateway.downloadCalls
	request.JobID = "second"
	result, err := service.Run(context.Background(), request, nil)
	if err != nil {
		t.Fatal(err)
	}
	if gateway.downloadCalls != calls {
		t.Fatalf("download called again for completed media: calls %d -> %d", calls, gateway.downloadCalls)
	}
	if result.Summary.AlreadyCompleted != 1 || result.Job.State != domain.JobCompleted {
		t.Fatalf("second result = %#v", result)
	}
}

func TestServiceRunAppliesDocumentFilterBeforeDownload(t *testing.T) {
	root := t.TempDir()
	gateway := newFakeGateway([]fakeMedia{
		{id: 3, name: "lesson.mp4", caption: "Lesson"},
		{id: 4, name: "slides.pdf", caption: "Slides"},
		{id: 5, name: "notes.docx", caption: "Notes"},
	})
	service, _ := newTestService(t, gateway)
	result, err := service.Run(context.Background(), RunRequest{
		JobID: "documents", Chat: domain.Chat{ID: 111, Type: "channel", Name: "Docs"},
		DestinationRoot: root, Filter: domain.FilterDocumentsOnly,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Job.Items) != 2 || result.Summary.Renamed != 2 {
		t.Fatalf("result = %#v", result)
	}
	if gateway.lastDownloadedItems != 2 {
		t.Fatalf("tdl received %d items, want 2", gateway.lastDownloadedItems)
	}
	if _, err := os.Stat(filepath.Join(root, "Docs [111]", "Videos", "Lesson.mp4")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("video should not be downloaded, stat err=%v", err)
	}
}

func TestServiceRunMarksExportFailureInPersistentJob(t *testing.T) {
	gateway := newFakeGateway([]fakeMedia{{id: 7, name: "lesson.mp4", caption: "Lesson"}})
	gateway.exportErr = errors.New("network unavailable")
	service, jobs := newTestService(t, gateway)
	_, err := service.Run(context.Background(), RunRequest{
		JobID: "fail-export", Chat: domain.Chat{ID: 222, Type: "channel", Name: "Failure"},
		DestinationRoot: t.TempDir(), Filter: domain.FilterVideosAndDocuments,
	}, nil)
	if err == nil || !strings.Contains(err.Error(), "network unavailable") {
		t.Fatalf("Run error = %v, want export error", err)
	}
	job, getErr := jobs.Get(context.Background(), "fail-export")
	if getErr != nil {
		t.Fatal(getErr)
	}
	if job.State != domain.JobFailed || job.ErrorMessage == "" {
		t.Fatalf("failed job not persisted: %#v", job)
	}
}

func TestServiceRunCompletesEmptyFilteredExportWithoutStartingDownload(t *testing.T) {
	root := t.TempDir()
	gateway := newFakeGateway([]fakeMedia{{id: 7, name: "lesson.mp4", caption: "Lesson"}})
	service, _ := newTestService(t, gateway)
	result, err := service.Run(context.Background(), RunRequest{
		JobID: "empty-filter", Chat: domain.Chat{ID: 333, Type: "channel", Name: "Empty"},
		DestinationRoot: root, Filter: domain.FilterDocumentsOnly,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if gateway.downloadCalls != 0 || result.Job.State != domain.JobCompleted {
		t.Fatalf("unexpected empty filtered run: calls=%d result=%#v", gateway.downloadCalls, result)
	}
}

func TestServiceRunCancellationPersistsCancelledState(t *testing.T) {
	root := t.TempDir()
	gateway := newFakeGateway([]fakeMedia{{id: 7, name: "lesson.mp4", caption: "Lesson"}})
	gateway.waitForCancel = true
	service, jobs := newTestService(t, gateway)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	finished := make(chan error, 1)
	go func() {
		_, err := service.Run(ctx, RunRequest{JobID: "cancel", Chat: domain.Chat{ID: 444, Type: "channel", Name: "Cancel"}, DestinationRoot: root, Filter: domain.FilterVideosAndDocuments}, nil)
		finished <- err
	}()
	select {
	case <-gateway.downloadStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("download did not start")
	}
	cancel()
	select {
	case err := <-finished:
		if err == nil || !errors.Is(err, context.Canceled) {
			t.Fatalf("Run error = %v, want context.Canceled", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not exit after cancellation")
	}
	job, err := jobs.Get(context.Background(), "cancel")
	if err != nil {
		t.Fatal(err)
	}
	if job.State != domain.JobCancelled {
		t.Fatalf("job state = %s, want cancelled", job.State)
	}
}

func TestServicePauseAndResumeReusesJobAndPendingExport(t *testing.T) {
	root := t.TempDir()
	gateway := newFakeGateway([]fakeMedia{{id: 7, name: "lesson.mp4", caption: "Lesson 1"}})
	gateway.waitForCancelOnCall = 1
	service, jobs := newTestService(t, gateway)
	request := RunRequest{
		JobID: "pause-resume", Chat: domain.Chat{ID: 880, Type: "channel", Name: "Lifecycle"},
		DestinationRoot: root, Filter: domain.FilterVideosAndDocuments, Takeout: true, GroupMedia: true,
	}
	ctx, cancel := context.WithCancelCause(context.Background())
	finished := make(chan struct {
		result RunResult
		err    error
	}, 1)
	go func() {
		result, err := service.Run(ctx, request, nil)
		finished <- struct {
			result RunResult
			err    error
		}{result, err}
	}()
	select {
	case <-gateway.downloadStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("download did not start")
	}
	cancel(ErrPauseRequested)
	var paused struct {
		result RunResult
		err    error
	}
	select {
	case paused = <-finished:
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not pause after cancellation cause")
	}
	if paused.err != nil {
		t.Fatalf("pause Run error = %v, want nil", paused.err)
	}
	if paused.result.Job.State != domain.JobPaused {
		t.Fatalf("pause result state = %s, want paused", paused.result.Job.State)
	}
	persisted, err := jobs.Get(context.Background(), request.JobID)
	if err != nil {
		t.Fatal(err)
	}
	if persisted.State != domain.JobPaused || !persisted.Takeout || !persisted.GroupMedia {
		t.Fatalf("persisted paused job lost state/options: %#v", persisted)
	}
	jobWorkDir := persisted.WorkDirectory
	if _, err := os.Stat(filepath.Join(jobWorkDir, "pending-export.json")); err != nil {
		t.Fatalf("pending export not preserved for resume: %v", err)
	}

	resumed, err := service.Resume(context.Background(), request.JobID, nil)
	if err != nil {
		t.Fatal(err)
	}
	if resumed.Job.State != domain.JobCompleted {
		t.Fatalf("resumed state = %s, want completed", resumed.Job.State)
	}
	if gateway.downloadCalls != 2 {
		t.Fatalf("download calls = %d, want 2", gateway.downloadCalls)
	}
	if !gateway.lastContinue {
		t.Fatal("resume did not request tdl --continue")
	}
	if gateway.exportCalls != 1 {
		t.Fatalf("export calls = %d, want original export reused on resume", gateway.exportCalls)
	}
	if _, err := os.Stat(filepath.Join(root, "Lifecycle [880]", "Videos", "Lesson 1.mp4")); err != nil {
		t.Fatalf("resumed media not saved: %v", err)
	}
}

func TestServiceResumeRejectsNonPausedJob(t *testing.T) {
	gateway := newFakeGateway([]fakeMedia{{id: 7, name: "lesson.mp4", caption: "Lesson 1"}})
	service, _ := newTestService(t, gateway)
	_, err := service.Run(context.Background(), RunRequest{
		JobID: "not-paused", Chat: domain.Chat{ID: 881, Type: "channel", Name: "Active"},
		DestinationRoot: t.TempDir(), Filter: domain.FilterVideosAndDocuments,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Resume(context.Background(), "not-paused", nil); err == nil || !strings.Contains(err.Error(), "only paused jobs can resume") {
		t.Fatalf("Resume error = %v, want a non-paused state error", err)
	}
}

func TestRecoverInterruptedJobsMarksActiveJobsPausedAndCancellingJobsCancelled(t *testing.T) {
	gateway := newFakeGateway(nil)
	service, jobs := newTestService(t, gateway)
	for _, job := range []domain.DownloadJob{
		{ID: "recover-download", Chat: domain.Chat{ID: 1, Name: "Download"}, State: domain.JobDownloading, Destination: filepath.Join(t.TempDir(), "Download"), Filter: domain.FilterVideosAndDocuments},
		{ID: "recover-preparing", Chat: domain.Chat{ID: 2, Name: "Preparing"}, State: domain.JobPreparing, Destination: filepath.Join(t.TempDir(), "Preparing"), Filter: domain.FilterVideosAndDocuments},
		{ID: "recover-cancel", Chat: domain.Chat{ID: 3, Name: "Cancelling"}, State: domain.JobCancelling, Destination: filepath.Join(t.TempDir(), "Cancelling"), Filter: domain.FilterVideosAndDocuments},
	} {
		if err := jobs.Save(context.Background(), job); err != nil {
			t.Fatal(err)
		}
	}
	recovered, err := service.RecoverInterruptedJobs(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if recovered != 3 {
		t.Fatalf("recovered = %d, want 3", recovered)
	}
	for _, item := range []struct {
		id   string
		want domain.JobState
	}{
		{"recover-download", domain.JobPaused},
		{"recover-preparing", domain.JobPaused},
		{"recover-cancel", domain.JobCancelled},
	} {
		job, err := jobs.Get(context.Background(), item.id)
		if err != nil {
			t.Fatal(err)
		}
		if job.State != item.want {
			t.Errorf("job %s state = %s, want %s", item.id, job.State, item.want)
		}
	}
}

func TestServiceSerializesTDLAcrossDifferentChats(t *testing.T) {
	gateway := newFakeGateway([]fakeMedia{{id: 1, name: "lesson.mp4", caption: "Lesson"}})
	gateway.waitForCancel = true
	service, _ := newTestService(t, gateway)
	root := t.TempDir()
	ctx1, cancel1 := context.WithCancel(context.Background())
	defer cancel1()
	firstDone := make(chan error, 1)
	go func() {
		_, err := service.Run(ctx1, RunRequest{JobID: "serialized-one", Chat: domain.Chat{ID: 1001, Type: "channel", Name: "One"}, DestinationRoot: root, Filter: domain.FilterVideosAndDocuments}, nil)
		firstDone <- err
	}()
	select {
	case <-gateway.downloadStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("first download did not start")
	}

	ctx2, cancel2 := context.WithCancel(context.Background())
	defer cancel2()
	secondDone := make(chan error, 1)
	go func() {
		_, err := service.Run(ctx2, RunRequest{JobID: "serialized-two", Chat: domain.Chat{ID: 1002, Type: "channel", Name: "Two"}, DestinationRoot: root, Filter: domain.FilterVideosAndDocuments}, nil)
		secondDone <- err
	}()
	time.Sleep(100 * time.Millisecond)
	gateway.mu.Lock()
	exportCallsWhileFirstActive := gateway.exportCalls
	gateway.mu.Unlock()
	if exportCallsWhileFirstActive != 1 {
		t.Fatalf("second tdl workflow started before first finished; export calls=%d, want 1", exportCallsWhileFirstActive)
	}

	cancel1()
	select {
	case <-firstDone:
	case <-time.After(2 * time.Second):
		t.Fatal("first job did not cancel")
	}
	select {
	case <-gateway.downloadStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("second queued job did not start after the first finished")
	}
	cancel2()
	select {
	case <-secondDone:
	case <-time.After(2 * time.Second):
		t.Fatal("second job did not cancel")
	}
}

func TestCancelPersistedPausedJobDoesNotNeedRunningTdlProcess(t *testing.T) {
	gateway := newFakeGateway(nil)
	service, jobs := newTestService(t, gateway)
	job := domain.DownloadJob{
		ID: "paused-to-cancel", Chat: domain.Chat{ID: 2001, Type: "channel", Name: "Paused"},
		State: domain.JobPaused, Destination: filepath.Join(t.TempDir(), "Paused [2001]"), Filter: domain.FilterVideosAndDocuments,
	}
	if err := jobs.Save(context.Background(), job); err != nil {
		t.Fatal(err)
	}
	var sawCancelled bool
	got, err := service.CancelPersistedJob(context.Background(), job.ID, func(event Event) {
		if event.Kind == "job_cancelled" {
			sawCancelled = true
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.State != domain.JobCancelled || !sawCancelled {
		t.Fatalf("cancel result = %#v, cancelled event=%v", got, sawCancelled)
	}
	if gateway.downloadCalls != 0 || gateway.exportCalls != 0 {
		t.Fatal("cancelling a paused job unexpectedly started tdl")
	}
}

func TestServiceSkipCompletedToggleAllowsReDownload(t *testing.T) {
	root := t.TempDir()
	gateway := newFakeGateway([]fakeMedia{{id: 17, name: "lesson.mp4", caption: "Lesson"}})
	service, _ := newTestService(t, gateway)
	request := RunRequest{
		Chat:            domain.Chat{ID: 791, Type: "channel", Name: "Toggle"},
		DestinationRoot: root, Filter: domain.FilterVideosAndDocuments, SkipSame: true,
	}
	request.JobID = "skip-on"
	if _, err := service.Run(context.Background(), request, nil); err != nil {
		t.Fatal(err)
	}
	request.JobID = "skip-off"
	request.SkipSame = false
	result, err := service.Run(context.Background(), request, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.Summary.Renamed != 1 || result.Summary.AlreadyCompleted != 0 {
		t.Fatalf("skip-off result summary = %#v", result.Summary)
	}
	if _, err := os.Stat(filepath.Join(root, "Toggle [791]", "Videos", "Lesson (2).mp4")); err != nil {
		t.Fatalf("force re-download did not preserve old file and add a duplicate-safe new file: %v", err)
	}
}

func TestResumeReconcilesFileMovedBeforeManifestCompletion(t *testing.T) {
	root := t.TempDir()
	chatRoot := filepath.Join(root, "Crash Gap [9920]")
	videoDir := filepath.Join(chatRoot, "Videos")
	if err := os.MkdirAll(videoDir, 0o700); err != nil {
		t.Fatal(err)
	}
	finalPath := filepath.Join(videoDir, "Lesson.mp4")
	if err := os.WriteFile(finalPath, []byte("already moved bytes"), 0o600); err != nil {
		t.Fatal(err)
	}

	item := domain.MediaItem{ChatID: 9920, MessageID: 17, OriginalName: "lesson.mp4", Caption: "Lesson"}
	manifestStore := filesystem.JSONManifestStore{}
	manifestPath := filepath.Join(chatRoot, ".telegram-media-manager-manifest.json")
	manifest := domain.NewManifest(9920, "Crash Gap")
	manifest.MarkMoving(item, "Lesson.mp4")
	if err := manifestStore.Save(context.Background(), manifestPath, manifest); err != nil {
		t.Fatal(err)
	}

	jobID := "reconcile-crash-gap"
	workDir := filepath.Join(chatRoot, ".tdl-work", workDirectoryName(jobID))
	if err := os.MkdirAll(filepath.Join(workDir, "staging"), 0o700); err != nil {
		t.Fatal(err)
	}
	pendingExport := filepath.Join(workDir, "pending-export.json")
	if err := os.WriteFile(pendingExport, []byte(`{"id":9920,"messages":[{"id":17,"type":"message","file":"lesson.mp4","text":"Lesson"}]}`), 0o600); err != nil {
		t.Fatal(err)
	}

	gateway := newFakeGateway([]fakeMedia{{id: 17, name: "lesson.mp4", caption: "Lesson"}})
	service, jobs := newTestService(t, gateway)
	job := domain.DownloadJob{
		ID: jobID, Chat: domain.Chat{ID: 9920, Type: "channel", Name: "Crash Gap"},
		State: domain.JobPaused, Destination: chatRoot, Filter: domain.FilterVideosAndDocuments,
		SkipSame: true, Items: []domain.MediaItem{item},
	}
	if err := jobs.Save(context.Background(), job); err != nil {
		t.Fatal(err)
	}

	result, err := service.Resume(context.Background(), jobID, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.Job.State != domain.JobCompleted {
		t.Fatalf("state = %s, want completed", result.Job.State)
	}
	if result.Summary.AlreadyCompleted != 1 {
		t.Fatalf("summary = %+v", result.Summary)
	}
	if gateway.downloadCalls != 0 {
		t.Fatalf("downloadCalls = %d, want no duplicate download", gateway.downloadCalls)
	}
	if content, readErr := os.ReadFile(finalPath); readErr != nil {
		t.Fatal(readErr)
	} else if string(content) != "already moved bytes" {
		t.Fatalf("final file changed: %q", string(content))
	}
	updated, err := manifestStore.Load(context.Background(), manifestPath, 9920, "Crash Gap")
	if err != nil {
		t.Fatal(err)
	}
	if updated.Items[item.Key()].Status != "completed" {
		t.Fatalf("manifest status = %q, want completed", updated.Items[item.Key()].Status)
	}
}

func newTestService(t *testing.T, gateway *fakeGateway) (*Service, *store.JSONJobStore) {
	t.Helper()
	jobs, err := store.NewJSONJobStore(filepath.Join(t.TempDir(), "state", "jobs.json"))
	if err != nil {
		t.Fatal(err)
	}
	service := NewService(gateway, jobs, filesystem.DownloadOrganizer{}, filesystem.JSONManifestStore{})
	return service, jobs
}

func hasEvent(events []Event, kind string) bool {
	for _, event := range events {
		if event.Kind == kind {
			return true
		}
	}
	return false
}

type fakeMedia struct {
	id      int64
	name    string
	caption string
}

type fakeGateway struct {
	items               []fakeMedia
	exportErr           error
	downloadErr         error
	downloadErrors      []error
	exportCalls         int
	downloadCalls       int
	lastDownloadedItems int
	lastContinue        bool
	waitForCancel       bool
	waitForCancelOnCall int
	downloadStarted     chan struct{}
	mu                  sync.Mutex
}

func newFakeGateway(items []fakeMedia) *fakeGateway {
	return &fakeGateway{items: items, downloadStarted: make(chan struct{}, 1)}
}

func (f *fakeGateway) Version(context.Context) (string, error) { return "tdl test", nil }
func (f *fakeGateway) ListChats(context.Context) ([]domain.Chat, error) {
	return []domain.Chat{{ID: 123, Type: "channel", Name: "Test"}}, nil
}
func (f *fakeGateway) ExportChat(ctx context.Context, chat domain.Chat, outputPath string) error {
	f.mu.Lock()
	f.exportCalls++
	f.mu.Unlock()
	if f.exportErr != nil {
		return f.exportErr
	}
	messages := make([]map[string]any, 0, len(f.items))
	for _, item := range f.items {
		messages = append(messages, map[string]any{"id": item.id, "type": "message", "file": item.name, "text": item.caption})
	}
	data, err := json.Marshal(map[string]any{"id": chat.ID, "messages": messages})
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o700); err != nil {
		return err
	}
	return os.WriteFile(outputPath, data, 0o600)
}
func (f *fakeGateway) Download(ctx context.Context, req domain.DownloadRequest, onProgress func(domain.ProgressEvent)) error {
	f.mu.Lock()
	f.downloadCalls++
	callNumber := f.downloadCalls
	f.lastContinue = req.Continue
	f.mu.Unlock()
	data, err := os.ReadFile(req.ExportPath)
	if err != nil {
		return err
	}
	items, err := tdl.ParseExport(data)
	if err != nil {
		// Subset exports in this fake may be represented by the same tdl export schema.
		return err
	}
	f.lastDownloadedItems = len(items)
	if f.waitForCancel || f.waitForCancelOnCall == callNumber {
		select {
		case f.downloadStarted <- struct{}{}:
		default:
		}
		<-ctx.Done()
		return ctx.Err()
	}
	if callNumber <= len(f.downloadErrors) && f.downloadErrors[callNumber-1] != nil {
		return f.downloadErrors[callNumber-1]
	}
	if f.downloadErr != nil {
		return f.downloadErr
	}
	if err := os.MkdirAll(req.Destination, 0o700); err != nil {
		return err
	}
	for _, item := range items {
		name := fmt.Sprintf("%d_%d_%s", item.ChatID, item.MessageID, item.OriginalName)
		if err := os.WriteFile(filepath.Join(req.Destination, name), []byte("test media"), 0o600); err != nil {
			return err
		}
	}
	if onProgress != nil {
		percent := 100.0
		onProgress(domain.ProgressEvent{JobID: req.JobID, Percent: &percent})
	}
	return nil
}
func (f *fakeGateway) ParseExport(data []byte) ([]domain.MediaItem, error) {
	return tdl.ParseExport(data)
}
func (f *fakeGateway) FilterExport(data []byte, items []domain.MediaItem) ([]byte, error) {
	return tdl.FilterExport(data, items)
}

var _ ports.TelegramGateway = (*fakeGateway)(nil)
var _ ports.ExportProcessor = (*fakeGateway)(nil)

func TestServiceRerunsWhenManifestedFileWasDeleted(t *testing.T) {
	root := t.TempDir()
	gateway := newFakeGateway([]fakeMedia{{id: 7, name: "lesson.mp4", caption: "Lesson 1"}})
	service, _ := newTestService(t, gateway)
	request := RunRequest{Chat: domain.Chat{ID: 790, Type: "channel", Name: "Replay"}, DestinationRoot: root, Filter: domain.FilterVideosAndDocuments, SkipSame: true}
	request.JobID = "first"
	if _, err := service.Run(context.Background(), request, nil); err != nil {
		t.Fatal(err)
	}
	videoPath := filepath.Join(root, "Replay [790]", "Videos", "Lesson 1.mp4")
	if err := os.Remove(videoPath); err != nil {
		t.Fatal(err)
	}
	request.JobID = "second"
	if _, err := service.Run(context.Background(), request, nil); err != nil {
		t.Fatal(err)
	}
	if gateway.downloadCalls != 2 {
		t.Fatalf("download calls = %d, want 2 after completed file was removed", gateway.downloadCalls)
	}
	if _, err := os.Stat(videoPath); err != nil {
		t.Fatalf("deleted media was not downloaded again: %v", err)
	}
}

func TestServiceRefusesUnavailableDestinationBeforeCreatingJob(t *testing.T) {
	gateway := newFakeGateway([]fakeMedia{{id: 1, name: "lesson.mp4", caption: "Lesson"}})
	service, jobs := newTestService(t, gateway)
	missing := filepath.Join(t.TempDir(), "external-drive-not-mounted")
	_, err := service.Run(context.Background(), RunRequest{
		JobID: "unavailable-destination", Chat: domain.Chat{ID: 991, Type: "channel", Name: "Unavailable"},
		DestinationRoot: missing, Filter: domain.FilterVideosAndDocuments,
	}, nil)
	if err == nil || !strings.Contains(err.Error(), "drive is mounted") {
		t.Fatalf("Run error = %v, want unavailable destination error", err)
	}
	if gateway.downloadCalls != 0 {
		t.Fatalf("download calls = %d, want 0", gateway.downloadCalls)
	}
	if _, err := jobs.Get(context.Background(), "unavailable-destination"); !errors.Is(err, ports.ErrJobNotFound) {
		t.Fatalf("job was saved despite invalid destination: %v", err)
	}
}

func TestServicePreventsConcurrentDownloadsForSameChat(t *testing.T) {
	gateway := newFakeGateway([]fakeMedia{{id: 1, name: "lesson.mp4", caption: "Lesson"}})
	gateway.waitForCancel = true
	service, _ := newTestService(t, gateway)
	root := t.TempDir()
	firstDone := make(chan error, 1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		_, err := service.Run(ctx, RunRequest{JobID: "active-one", Chat: domain.Chat{ID: 992, Type: "channel", Name: "Busy"}, DestinationRoot: root, Filter: domain.FilterVideosAndDocuments}, nil)
		firstDone <- err
	}()
	select {
	case <-gateway.downloadStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("first download did not start")
	}
	_, err := service.Run(context.Background(), RunRequest{JobID: "active-two", Chat: domain.Chat{ID: 992, Type: "channel", Name: "Busy"}, DestinationRoot: root, Filter: domain.FilterVideosAndDocuments}, nil)
	if err == nil || !strings.Contains(err.Error(), "already active") {
		t.Fatalf("second Run error = %v, want per-chat concurrency error", err)
	}
	cancel()
	select {
	case <-firstDone:
	case <-time.After(2 * time.Second):
		t.Fatal("first download did not finish after cancellation")
	}
}

func TestServiceRetriesTransientDownloadFailureWithContinuation(t *testing.T) {
	root := t.TempDir()
	gateway := newFakeGateway([]fakeMedia{{id: 18, name: "lecture.mp4", caption: "Lecture"}})
	gateway.downloadErrors = []error{
		&domain.AppError{Code: domain.ErrorCodeUnavailable, Message: "temporary network failure", Retryable: true},
	}
	service, jobs := newTestService(t, gateway)
	var events []Event
	result, err := service.Run(context.Background(), RunRequest{
		JobID: "retry-transient", Chat: domain.Chat{ID: 903, Type: "channel", Name: "Retry Course"},
		DestinationRoot: root, Filter: domain.FilterVideosAndDocuments,
	}, func(event Event) { events = append(events, event) })
	if err != nil {
		t.Fatal(err)
	}
	if result.Job.State != domain.JobCompleted || gateway.downloadCalls != 2 {
		t.Fatalf("result=%+v calls=%d, want completed after 2 attempts", result, gateway.downloadCalls)
	}
	if !gateway.lastContinue {
		t.Fatal("second download attempt should use continuation")
	}
	if !hasEvent(events, "download_retry") {
		t.Fatalf("missing download_retry event: %#v", events)
	}
	persisted, err := jobs.Get(context.Background(), "retry-transient")
	if err != nil {
		t.Fatal(err)
	}
	if persisted.State != domain.JobCompleted {
		t.Fatalf("persisted state = %q", persisted.State)
	}
}

func TestServiceDoesNotRetryPermanentDownloadFailure(t *testing.T) {
	gateway := newFakeGateway([]fakeMedia{{id: 19, name: "lecture.mp4", caption: "Lecture"}})
	gateway.downloadErrors = []error{
		&domain.AppError{Code: domain.ErrorCodeUnauthorized, Message: "session expired", Retryable: false},
	}
	service, _ := newTestService(t, gateway)
	_, err := service.Run(context.Background(), RunRequest{
		JobID: "no-retry-auth", Chat: domain.Chat{ID: 904, Type: "channel", Name: "Auth Course"},
		DestinationRoot: t.TempDir(), Filter: domain.FilterVideosAndDocuments,
	}, nil)
	if err == nil {
		t.Fatal("Run() error = nil, want auth failure")
	}
	if gateway.downloadCalls != 1 {
		t.Fatalf("download calls=%d, want 1 for permanent error", gateway.downloadCalls)
	}
}

func TestServicePersistsTypedFailureCode(t *testing.T) {
	gateway := newFakeGateway([]fakeMedia{{id: 21, name: "secret.pdf", caption: "Secret"}})
	gateway.downloadErrors = []error{&domain.AppError{Code: domain.ErrorCodeUnauthorized, Message: "session expired", Retryable: false}}
	service, jobs := newTestService(t, gateway)
	_, err := service.Run(context.Background(), RunRequest{
		JobID: "typed-auth-failure", Chat: domain.Chat{ID: 906, Type: "channel", Name: "Auth Failure"},
		DestinationRoot: t.TempDir(), Filter: domain.FilterVideosAndDocuments,
	}, nil)
	if err == nil {
		t.Fatal("expected download error")
	}
	job, getErr := jobs.Get(context.Background(), "typed-auth-failure")
	if getErr != nil {
		t.Fatal(getErr)
	}
	if job.ErrorCode != string(domain.ErrorCodeUnauthorized) {
		t.Fatalf("ErrorCode=%q, want %q", job.ErrorCode, domain.ErrorCodeUnauthorized)
	}
}
