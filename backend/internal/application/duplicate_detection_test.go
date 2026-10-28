package application

import (
	"context"
	"encoding/json"
	"example.com/telegram-media-downloader/backend/internal/domain"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type identityGateway struct {
	*libraryGateway
	documentID int64
	caption    string
}

func (g *identityGateway) ExportMedia(_ context.Context, q domain.MediaQuery, path string) error {
	g.exportCalls++
	raw := fmt.Sprintf(`{"id":%d,"messages":[{"id":7,"file":"lesson.mp4","text":%q,"raw":{"Media":{"Document":{"ID":%d,"Size":10}}}}]}`, q.Chat.ID, g.caption, g.documentID)
	return os.WriteFile(path, []byte(raw), 0600)
}
func identityService(t *testing.T) (*Service, *identityGateway) {
	t.Helper()
	fake := newFakeGateway(nil)
	s, _ := newTestService(t, fake)
	gateway := &identityGateway{libraryGateway: &libraryGateway{fakeGateway: fake}, documentID: 9223372036854775000, caption: "Original caption"}
	s.Telegram = gateway
	return s, gateway
}
func runIdentityMedia(s *Service, id, root string, chat domain.Chat) (RunResult, error) {
	return s.RunMedia(context.Background(), MediaRunRequest{JobID: id, Chat: chat, DestinationRoot: root, Selection: domain.MediaSelection{EntireScope: true, FileTypes: []string{"video"}}}, nil)
}
func TestCaptionEditsKeepSavedMediaAcrossRestart(t *testing.T) {
	s, g := identityService(t)
	root := t.TempDir()
	chat := domain.Chat{ID: 123, Name: "Course"}
	first, err := runIdentityMedia(s, "first", root, chat)
	if err != nil {
		t.Fatal(err)
	}
	g.caption = "Edited caption"
	fresh := NewService(g, s.Jobs, s.Organizer, s.Manifests)
	second, err := runIdentityMedia(fresh, "second", root, chat)
	if err != nil || g.downloadCalls != 1 || second.Job.Files[0].SavedPath != first.Job.Files[0].SavedPath {
		t.Fatalf("caption edit downloaded again: calls=%d err=%v", g.downloadCalls, err)
	}
	if _, err := os.Stat(filepath.Join(first.Job.Destination, "Videos", "Edited caption.mp4")); !os.IsNotExist(err) {
		t.Fatal("caption edit renamed/duplicated saved file")
	}
}
func TestRenamedChatReusesStableFolderWithOrWithoutJobHistory(t *testing.T) {
	for _, removeHistory := range []bool{false, true} {
		t.Run(fmt.Sprint(removeHistory), func(t *testing.T) {
			s, g := identityService(t)
			root := t.TempDir()
			chat := domain.Chat{ID: 123, Name: "Original Course"}
			first, err := runIdentityMedia(s, "first", root, chat)
			if err != nil {
				t.Fatal(err)
			}
			if removeHistory {
				if err := s.Jobs.Delete(context.Background(), "first"); err != nil {
					t.Fatal(err)
				}
			}
			chat.Name = "Renamed Course"
			fresh := NewService(g, s.Jobs, s.Organizer, s.Manifests)
			second, err := runIdentityMedia(fresh, "second", root, chat)
			if err != nil || g.downloadCalls != 1 || second.Job.Destination != first.Job.Destination {
				t.Fatalf("renamed chat split storage: first=%s next=%s calls=%d err=%v", first.Job.Destination, second.Job.Destination, g.downloadCalls, err)
			}
			if _, err := os.Stat(filepath.Join(root, "Renamed Course [123]")); !os.IsNotExist(err) {
				t.Fatal("created duplicate chat directory")
			}
		})
	}
}
func TestKnownMediaReplacementDownloadsEvenWithSameNameAndSize(t *testing.T) {
	s, g := identityService(t)
	root := t.TempDir()
	chat := domain.Chat{ID: 123, Name: "Course"}
	first, err := runIdentityMedia(s, "first", root, chat)
	if err != nil {
		t.Fatal(err)
	}
	g.documentID = 9223372036854775001
	second, err := runIdentityMedia(s, "second", root, chat)
	if err != nil || g.downloadCalls != 2 || second.Job.Files[0].SavedPath == first.Job.Files[0].SavedPath {
		t.Fatalf("replacement skipped/overwritten: calls=%d err=%v", g.downloadCalls, err)
	}
	if data, err := os.ReadFile(first.Job.Files[0].SavedPath); err != nil || string(data) != "test media" {
		t.Fatal("old saved file overwritten")
	}
}
func TestCaptionEditWithLegacyManifestStillSkipsExistingFile(t *testing.T) {
	s, g := identityService(t)
	root := t.TempDir()
	chat := domain.Chat{ID: 123, Name: "Course"}
	first, err := runIdentityMedia(s, "first", root, chat)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(first.Job.Destination, ".telegram-media-manager-manifest.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var manifest map[string]any
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatal(err)
	}
	for _, raw := range manifest["items"].(map[string]any) {
		delete(raw.(map[string]any), "contentId")
	}
	data, _ = json.Marshal(manifest)
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	g.caption = "Edited"
	result, err := runIdentityMedia(s, "legacy", root, chat)
	if err != nil || g.downloadCalls != 1 || result.Job.Files[0].SavedPath != first.Job.Files[0].SavedPath {
		t.Fatalf("legacy caption caused duplicate: calls=%d err=%v", g.downloadCalls, err)
	}
}
func TestMissingOrWrongSizeSavedMediaIsDownloadedAgain(t *testing.T) {
	for _, missing := range []bool{false, true} {
		t.Run(fmt.Sprint(missing), func(t *testing.T) {
			s, g := identityService(t)
			root := t.TempDir()
			chat := domain.Chat{ID: 123, Name: "Course"}
			first, err := runIdentityMedia(s, "first", root, chat)
			if err != nil {
				t.Fatal(err)
			}
			path := first.Job.Files[0].SavedPath
			if missing {
				err = os.Remove(path)
			} else {
				err = os.WriteFile(path, []byte("broken"), 0600)
			}
			if err != nil {
				t.Fatal(err)
			}
			g.caption = "Edited"
			second, err := runIdentityMedia(s, "repair", root, chat)
			if err != nil || g.downloadCalls != 2 || second.Job.Files[0].State != "completed" {
				t.Fatalf("missing/corrupt file skipped: %v", err)
			}
		})
	}
}
func TestDuplicateDetectionNeverCrossesDestinationOrChat(t *testing.T) {
	s, g := identityService(t)
	root := t.TempDir()
	chat := domain.Chat{ID: 123, Name: "Course"}
	if _, err := runIdentityMedia(s, "first", root, chat); err != nil {
		t.Fatal(err)
	}
	if _, err := runIdentityMedia(s, "other-destination", t.TempDir(), chat); err != nil {
		t.Fatal(err)
	}
	chat.ID = 456
	if _, err := runIdentityMedia(s, "other-chat", root, chat); err != nil {
		t.Fatal(err)
	}
	if g.downloadCalls != 3 {
		t.Fatalf("different destinations/chats shared saved media: %d", g.downloadCalls)
	}
}
func TestConflictingSavedChatFoldersRequireResolution(t *testing.T) {
	s, g := identityService(t)
	root := t.TempDir()
	chat := domain.Chat{ID: 123, Name: "Course"}
	first, err := runIdentityMedia(s, "first", root, chat)
	if err != nil {
		t.Fatal(err)
	}
	other := filepath.Join(root, "Another Course [123]")
	if err := os.MkdirAll(other, 0700); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(first.Job.Destination, ".telegram-media-manager-manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(other, ".telegram-media-manager-manifest.json"), data, 0600); err != nil {
		t.Fatal(err)
	}
	fresh := NewService(g, s.Jobs, s.Organizer, s.Manifests)
	chat.Name = "Renamed"
	_, err = runIdentityMedia(fresh, "conflict", root, chat)
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "multiple") || g.downloadCalls != 1 {
		t.Fatalf("ambiguous roots silently chosen: calls=%d err=%v", g.downloadCalls, err)
	}
}

func TestLegacyWorkflowKeepsFolderAndFileAfterMetadataEdits(t *testing.T) {
	g := newFakeGateway([]fakeMedia{{id: 7, name: "lesson.mp4", caption: "Original"}})
	s, _ := newTestService(t, g)
	request := RunRequest{JobID: "legacy-first", Chat: domain.Chat{ID: 123, Name: "Course"}, DestinationRoot: t.TempDir(), SkipSame: true}
	first, err := s.Run(context.Background(), request, nil)
	if err != nil {
		t.Fatal(err)
	}
	g.items[0].caption = "Edited"
	request.Chat.Name = "Renamed"
	request.JobID = "legacy-second"
	fresh := NewService(g, s.Jobs, s.Organizer, s.Manifests)
	second, err := fresh.Run(context.Background(), request, nil)
	if err != nil || g.downloadCalls != 1 || second.Job.Destination != first.Job.Destination || second.Summary.AlreadyCompleted != 1 {
		t.Fatalf("legacy metadata duplicated output: calls=%d summary=%+v err=%v", g.downloadCalls, second.Summary, err)
	}
}
func TestFolderChoiceIsStableBeforeQueuedJobHasBeenSaved(t *testing.T) {
	s, _ := identityService(t)
	root := t.TempDir()
	results := make(chan string, 20)
	errs := make(chan error, 20)
	for i := 0; i < 20; i++ {
		go func(index int) {
			dirs, err := s.resolveChatDirectories(context.Background(), root, domain.Chat{ID: 123, Name: fmt.Sprintf("Name %d", index)})
			results <- dirs.Root
			errs <- err
		}(i)
	}
	chosen := ""
	for i := 0; i < 20; i++ {
		path := <-results
		if err := <-errs; err != nil {
			t.Fatal(err)
		}
		if chosen == "" {
			chosen = path
		}
		if path != chosen {
			t.Fatalf("queued names split chat: %s vs %s", chosen, path)
		}
	}
}
func TestSavedFolderSymlinkIsRejectedBeforeDownloading(t *testing.T) {
	s, g := identityService(t)
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "Course [123]")); err != nil {
		t.Skip(err)
	}
	_, err := runIdentityMedia(s, "symlink", root, domain.Chat{ID: 123, Name: "Course"})
	if err == nil || g.downloadCalls != 0 {
		t.Fatalf("followed saved-folder symlink: calls=%d err=%v", g.downloadCalls, err)
	}
}
func TestCorruptSavedChatIdentityDoesNotCreateAnotherFolder(t *testing.T) {
	s, g := identityService(t)
	root := t.TempDir()
	folder := filepath.Join(root, "Original [123]")
	if err := os.MkdirAll(folder, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(folder, chatManifestName), []byte("corrupt"), 0600); err != nil {
		t.Fatal(err)
	}
	_, err := runIdentityMedia(s, "corrupt", root, domain.Chat{ID: 123, Name: "Renamed"})
	if err == nil || g.downloadCalls != 0 {
		t.Fatalf("ignored damaged identity: calls=%d err=%v", g.downloadCalls, err)
	}
}

func TestLegacyResumeAfterChatRenameUsesOriginalWorkDirectory(t *testing.T) {
	g := newFakeGateway([]fakeMedia{{id: 7, name: "lesson.mp4", caption: "Lesson"}})
	g.waitForCancel = true
	s, _ := newTestService(t, g)
	request := RunRequest{JobID: "rename-resume", Chat: domain.Chat{ID: 123, Name: "Original"}, DestinationRoot: t.TempDir(), SkipSame: true}
	ctx, cancel := context.WithCancelCause(context.Background())
	done := make(chan RunResult, 1)
	go func() {
		result, err := s.Run(ctx, request, nil)
		if err != nil {
			t.Errorf("pause: %v", err)
		}
		done <- result
	}()
	select {
	case <-g.downloadStarted:
	case <-time.After(3 * time.Second):
		t.Fatal("download did not start")
	}
	cancel(ErrPauseRequested)
	var paused RunResult
	select {
	case paused = <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("pause did not finish")
	}
	if paused.Job.State != domain.JobPaused {
		t.Fatalf("state=%s", paused.Job.State)
	}
	g.waitForCancel = false
	request.Chat.Name = "Renamed"
	fresh := NewService(g, s.Jobs, s.Organizer, s.Manifests)
	resumed, err := fresh.Run(context.Background(), request, nil)
	if err != nil || resumed.Job.State != domain.JobCompleted || resumed.Job.Destination != paused.Job.Destination || !g.lastContinue {
		t.Fatalf("resume abandoned original work: job=%+v err=%v", resumed.Job, err)
	}
}
