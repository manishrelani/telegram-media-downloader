package application

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"example.com/telegram-media-downloader/backend/internal/safefs"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"example.com/telegram-media-downloader/backend/internal/domain"
	"example.com/telegram-media-downloader/backend/internal/ports"
)

type scopedExporter interface {
	ExportMedia(context.Context, domain.MediaQuery, string) error
}

type MediaPage struct {
	Items      []domain.MediaItem `json:"items"`
	SnapshotID string             `json:"snapshotId"`
	HasMore    bool               `json:"hasMore"`
	Cached     bool               `json:"cached"`
	LoadedAt   int64              `json:"loadedAt"`
}
type mediaSnapshot struct {
	Query    domain.MediaQuery
	Page     MediaPage
	Data     []byte
	StoredAt int64
}
type MediaRunRequest struct {
	JobID           string
	Chat            domain.Chat
	DestinationRoot string
	Selection       domain.MediaSelection
	Takeout         bool
}

func sameScope(a, b domain.MediaQuery) bool {
	var at, bt int64
	if a.Topic != nil {
		at = a.Topic.ID
	}
	if b.Topic != nil {
		bt = b.Topic.ID
	}
	return a.Chat.ID == b.Chat.ID && at == bt
}

// Cached pages remain usable while a transfer holds the CLI's session database.
// Uncached discovery waits for the same slot as downloads; the app handler runs
// this asynchronously so Pause and Cancel remain responsive.
func (s *Service) ListMedia(ctx context.Context, query domain.MediaQuery) (MediaPage, error) {
	s.cancelPreviews()
	if query.Chat.ID == 0 || query.Limit < 0 || query.Limit > 1000000 || (query.Topic != nil && query.Topic.ID <= 0) {
		return MediaPage{}, fmt.Errorf("invalid media query")
	}
	s.libraryMu.Lock()
	if !query.Refresh {
		var newest *mediaSnapshot
		for _, snapshot := range s.snapshots {
			if sameScope(query, snapshot.Query) && query.Limit == snapshot.Query.Limit {
				if newest == nil || snapshot.StoredAt > newest.StoredAt {
					copy := snapshot
					newest = &copy
				}
			}
		}
		if newest != nil {
			page := newest.Page
			page.Cached = true
			s.libraryMu.Unlock()
			return page, nil
		}
	}
	s.libraryMu.Unlock()
	release, err := s.acquireTDL(ctx)
	if err != nil {
		return MediaPage{}, err
	}
	defer release()
	return s.exportMediaPage(ctx, query)
}

// Caller owns the tdl slot.
func (s *Service) exportMediaPage(ctx context.Context, query domain.MediaQuery) (MediaPage, error) {
	exporter, ok := s.Telegram.(scopedExporter)
	if !ok {
		return MediaPage{}, fmt.Errorf("downloader does not support media browsing")
	}
	dir, err := os.MkdirTemp("", "tmm-media-*")
	if err != nil {
		return MediaPage{}, err
	}
	defer os.RemoveAll(dir)
	upstream := query
	if upstream.Limit > 0 {
		upstream.Limit++
	}
	if err := exporter.ExportMedia(ctx, upstream, filepath.Join(dir, "export.json")); err != nil {
		return MediaPage{}, err
	}
	data, err := os.ReadFile(filepath.Join(dir, "export.json"))
	if err != nil {
		return MediaPage{}, err
	}
	items, err := s.Telegram.ParseExport(data)
	if err != nil {
		return MediaPage{}, err
	}
	for i := range items {
		if items[i].ChatID != query.Chat.ID {
			return MediaPage{}, fmt.Errorf("export belongs to another chat")
		}
		if query.Topic != nil {
			items[i].TopicID = query.Topic.ID
		}
	}
	sort.SliceStable(items, func(i, j int) bool { return items[i].MessageID > items[j].MessageID })
	more := query.Limit > 0 && len(items) > query.Limit
	if more {
		items = items[:query.Limit]
	}
	identity := sha256.Sum256(append([]byte(fmt.Sprintf("%d:%v:%d:", query.Chat.ID, topicID(query.Topic), query.Limit)), data...))
	page := MediaPage{Items: items, HasMore: more, SnapshotID: hex.EncodeToString(identity[:]), LoadedAt: time.Now().UnixMilli()}
	s.libraryMu.Lock()
	defer s.libraryMu.Unlock()
	if s.snapshots == nil {
		s.snapshots = make(map[string]mediaSnapshot)
	}
	// Bound retained raw metadata. Jobs copy their immutable selection to disk.
	var total int
	for _, snapshot := range s.snapshots {
		total += len(snapshot.Data)
	}
	for len(s.snapshots) >= 16 || (len(s.snapshots) > 0 && total+len(data) > 64*1024*1024) {
		oldest := ""
		var at int64
		for key, snapshot := range s.snapshots {
			if oldest == "" || snapshot.Page.LoadedAt < at {
				oldest = key
				at = snapshot.Page.LoadedAt
			}
		}
		total -= len(s.snapshots[oldest].Data)
		delete(s.snapshots, oldest)
	}
	s.snapshots[page.SnapshotID] = mediaSnapshot{Query: query, Page: page, Data: data, StoredAt: time.Now().UnixNano()}
	return page, nil
}

func topicID(topic *domain.Topic) int64 {
	if topic == nil {
		return 0
	}
	return topic.ID
}

func (s *Service) ListChats(ctx context.Context) ([]domain.Chat, error) {
	select {
	case s.tdlSlot <- struct{}{}:
		defer func() { <-s.tdlSlot }()
	default:
		s.libraryMu.Lock()
		cached := append([]domain.Chat(nil), s.chatCache...)
		s.libraryMu.Unlock()
		if cached != nil {
			return cached, nil
		}
		return nil, &domain.AppError{Code: domain.ErrorCodeUnavailable, Message: "Downloader is busy. Pause the current download before refreshing chats."}
	}
	chats, err := s.Telegram.ListChats(ctx)
	if err == nil {
		s.libraryMu.Lock()
		s.chatCache = chats
		s.libraryMu.Unlock()
	}
	return chats, err
}

func mediaWorkDir(job domain.DownloadJob) string {
	if job.WorkDirectory != "" {
		return job.WorkDirectory
	}
	return filepath.Join(job.Destination, ".tdl-work", workDirectoryName(job.ID))
}

func (s *Service) WithTDL(ctx context.Context, run func() error) error {
	release, err := s.acquireTDL(ctx)
	if err != nil {
		return err
	}
	defer release()
	return run()
}

func (s *Service) RunMedia(ctx context.Context, request MediaRunRequest, emit func(Event)) (RunResult, error) {
	if err := s.validate(RunRequest{JobID: request.JobID, Chat: request.Chat, DestinationRoot: request.DestinationRoot, Filter: domain.FilterAllAttachments}); err != nil {
		return RunResult{}, err
	}
	if err := domain.ValidateFileTypes(request.Selection.FileTypes); err != nil {
		return RunResult{}, err
	}
	if request.Selection.Topic != nil && request.Selection.Topic.ID <= 0 {
		return RunResult{}, fmt.Errorf("invalid topic")
	}
	if _, err := s.Jobs.Get(ctx, request.JobID); err == nil {
		return RunResult{}, fmt.Errorf("job ID already exists")
	} else if !errors.Is(err, ports.ErrJobNotFound) {
		return RunResult{}, err
	}
	dirs, err := s.resolveChatDirectories(ctx, request.DestinationRoot, request.Chat)
	if err != nil {
		return RunResult{}, err
	}
	job := domain.DownloadJob{ID: request.JobID, Chat: request.Chat, Library: true, State: domain.JobQueued, Destination: dirs.Root, DestinationRoot: request.DestinationRoot, Filter: domain.FilterAllAttachments, Topic: request.Selection.Topic, FileTypes: request.Selection.FileTypes, Selection: &request.Selection, Takeout: request.Takeout, SkipSame: true}
	work, err := s.newWorkDirectory()
	if err != nil {
		return RunResult{}, err
	}
	job.WorkDirectory = work
	var data []byte
	if !request.Selection.EntireScope {
		s.libraryMu.Lock()
		snapshot, ok := s.snapshots[request.Selection.SnapshotID]
		s.libraryMu.Unlock()
		if !ok {
			return RunResult{}, fmt.Errorf("media list expired; refresh it and select the files again")
		}
		if !sameScope(snapshot.Query, domain.MediaQuery{Chat: request.Chat, Topic: request.Selection.Topic}) {
			return RunResult{}, fmt.Errorf("selection does not belong to this chat/topic")
		}
		allowed := make(map[string]domain.MediaItem)
		for _, item := range snapshot.Page.Items {
			allowed[item.Key()] = item
		}
		seen := make(map[string]bool)
		for _, key := range request.Selection.Keys {
			item, ok := allowed[key]
			if !ok || !domain.MatchesFileTypes(item.OriginalName, job.FileTypes) {
				return RunResult{}, fmt.Errorf("selected file is outside the current scope or file filters")
			}
			if !seen[key] {
				job.Items = append(job.Items, item)
				job.Files = append(job.Files, domain.FileTask{Item: item, State: "queued"})
				seen[key] = true
			}
		}
		if len(job.Files) == 0 {
			return RunResult{}, fmt.Errorf("select at least one file")
		}
		data, err = s.Telegram.FilterExport(snapshot.Data, job.Items)
		if err != nil {
			return RunResult{}, err
		}
	}
	if err := safefs.Ensure(workBoundary(job), mediaWorkDir(job)); err != nil {
		return RunResult{}, err
	}
	if data != nil {
		if err := os.WriteFile(filepath.Join(mediaWorkDir(job), "export.json"), data, 0o600); err != nil {
			return RunResult{}, err
		}
	}
	if err := s.saveMediaJob(&job, emit); err != nil {
		return RunResult{}, err
	}
	return s.executeMedia(ctx, job, emit)
}

func (s *Service) RetryMedia(ctx context.Context, jobID string, emit func(Event)) (RunResult, error) {
	job, err := s.Jobs.Get(ctx, jobID)
	if err != nil {
		return RunResult{}, err
	}
	if !job.Library || (job.State != domain.JobFailed && job.State != domain.JobCancelled) {
		return RunResult{}, fmt.Errorf("only failed/cancelled file downloads can be retried")
	}
	return s.continueMedia(ctx, job, true, emit)
}

func (s *Service) continueMedia(ctx context.Context, job domain.DownloadJob, retry bool, emit func(Event)) (RunResult, error) {
	if err := s.ensurePrivateWork(ctx, &job); err != nil {
		return RunResult{}, err
	}
	if info, err := os.Stat(job.DestinationRoot); err != nil || !info.IsDir() {
		return RunResult{}, fmt.Errorf("original destination is unavailable; reconnect the drive and retry")
	}
	if retry {
		next, err := job.Transition(domain.JobQueued)
		if err != nil {
			return RunResult{}, err
		}
		job = next
	}
	job.ErrorCode = ""
	job.ErrorMessage = ""
	for i := range job.Files {
		if job.Files[i].State != "completed" {
			job.Files[i].State = "queued"
			job.Files[i].Error = ""
		}
	}
	if err := s.saveMediaJob(&job, emit); err != nil {
		return RunResult{}, err
	}
	return s.executeMedia(ctx, job, emit)
}

func (s *Service) saveMediaJob(job *domain.DownloadJob, emit func(Event)) error {
	if err := s.Jobs.Save(context.Background(), *job); err != nil {
		return err
	}
	copy := *job
	copy.Files = append([]domain.FileTask(nil), job.Files...)
	emitEvent(emit, Event{Kind: "job_state", Job: &copy})
	return nil
}

func (s *Service) executeMedia(ctx context.Context, job domain.DownloadJob, emit func(Event)) (RunResult, error) {
	// Wait as queued; one slot covers both Telegram reads and file transfers.
	release, err := s.acquireTDL(ctx)
	if err != nil {
		return s.stopMedia(ctx, job, err, emit)
	}
	if err := s.transition(context.Background(), &job, domain.JobPreparing, emit); err != nil {
		release()
		return RunResult{Job: job}, err
	}
	exportPath := filepath.Join(mediaWorkDir(job), "export.json")
	data, readErr := os.ReadFile(exportPath)
	if readErr != nil && len(job.Files) == 0 && job.Selection != nil && job.Selection.EntireScope {
		page, err := s.exportMediaPage(ctx, domain.MediaQuery{Chat: job.Chat, Topic: job.Topic, Limit: 0, Refresh: true})
		if err != nil {
			release()
			return s.stopMedia(ctx, job, err, emit)
		}
		s.libraryMu.Lock()
		snapshot := s.snapshots[page.SnapshotID]
		s.libraryMu.Unlock()
		for _, item := range page.Items {
			if domain.MatchesFileTypes(item.OriginalName, job.FileTypes) {
				job.Items = append(job.Items, item)
				job.Files = append(job.Files, domain.FileTask{Item: item, State: "queued"})
			}
		}
		data, err = s.Telegram.FilterExport(snapshot.Data, job.Items)
		if err == nil {
			err = os.WriteFile(exportPath, data, 0o600)
		}
		if err != nil {
			release()
			return s.stopMedia(ctx, job, err, emit)
		}
	} else if readErr != nil {
		release()
		return s.stopMedia(ctx, job, fmt.Errorf("saved selection is unavailable: %w", readErr), emit)
	}
	// Recover a crash after saving the export but before saving discovered rows.
	if len(job.Files) == 0 && job.Selection != nil && job.Selection.EntireScope {
		items, err := s.Telegram.ParseExport(data)
		if err != nil {
			release()
			return s.stopMedia(ctx, job, err, emit)
		}
		for _, item := range items {
			if job.Topic != nil {
				item.TopicID = job.Topic.ID
			}
			if domain.MatchesFileTypes(item.OriginalName, job.FileTypes) {
				job.Items = append(job.Items, item)
				job.Files = append(job.Files, domain.FileTask{Item: item, State: "queued"})
			}
		}
	}
	if err := s.saveMediaJob(&job, emit); err != nil {
		release()
		return s.stopMedia(ctx, job, err, emit)
	}
	release()
	if len(job.Files) > 0 {
		if err := s.transition(context.Background(), &job, domain.JobDownloading, emit); err != nil {
			return RunResult{Job: job}, err
		}
	}
	for i := range job.Files {
		if ctx.Err() != nil {
			return s.stopMedia(ctx, job, ctx.Err(), emit)
		}
		release, err = s.acquireTDL(ctx)
		if err != nil {
			return s.stopMedia(ctx, job, err, emit)
		}
		err = s.transferFile(ctx, &job, i, data, emit)
		release()
		if ctx.Err() != nil {
			return s.stopMedia(ctx, job, err, emit)
		}
		if err != nil {
			job.Files[i].State = "failed"
			job.Files[i].Error = truncateError(err.Error(), 500)
			if saveErr := s.saveMediaJob(&job, emit); saveErr != nil {
				return s.stopMedia(ctx, job, saveErr, emit)
			}
			// Storage/authentication problems need intervention; leave later files queued.
			var appErr *domain.AppError
			if errors.As(err, &appErr) && (appErr.Code == domain.ErrorCodeStorage || appErr.Code == domain.ErrorCodeUnauthorized || appErr.Code == domain.ErrorCodeRateLimited) {
				return s.stopMedia(ctx, job, err, emit)
			}
		}
	}
	failed := 0
	for _, file := range job.Files {
		if file.State != "completed" {
			failed++
		}
	}
	if failed > 0 {
		return s.stopMedia(ctx, job, fmt.Errorf("%d file(s) did not complete; retry to continue only unfinished files", failed), emit)
	}
	if err := s.transition(context.Background(), &job, domain.JobCompleted, emit); err != nil {
		return RunResult{Job: job}, err
	}
	emitEvent(emit, Event{Kind: "job_completed", Job: jobPointer(job), Message: fmt.Sprintf("%d files saved", len(job.Files))})
	return RunResult{Job: job}, nil
}

// Caller owns the shared slot. A single-item input gives progress an unambiguous
// identity, even if the terminal renderer truncates the filename or omits it.
func (s *Service) transferFile(ctx context.Context, job *domain.DownloadJob, index int, data []byte, emit func(Event)) error {
	file := &job.Files[index]
	item := file.Item
	previouslyCompleted := file.State == "completed"
	if info, err := os.Stat(job.DestinationRoot); err != nil || !info.IsDir() {
		return storageFailure(fmt.Errorf("save destination is unavailable; reconnect the drive before retrying"))
	}
	if err := safefs.Ensure(job.DestinationRoot, job.Destination); err != nil {
		return storageFailure(err)
	}
	manifestPath := filepath.Join(job.Destination, ".telegram-media-manager-manifest.json")
	check, _, checkErr := safefs.Open(job.DestinationRoot, manifestPath)
	if checkErr != nil {
		return storageFailure(checkErr)
	}
	check.Close()
	manifest, err := s.Manifests.Load(ctx, manifestPath, job.Chat.ID, job.Chat.Name)
	if err != nil {
		return storageFailure(err)
	}
	if entry, ok := manifest.Items[item.Key()]; ok && (entry.Status == "completed" || entry.Status == "moving") {
		present, err := completedFilePresent(job.Destination, item, entry)
		if err != nil {
			return storageFailure(err)
		}
		if present {
			entry.Status = "completed"
			manifest.Items[item.Key()] = entry
			if err := s.Manifests.Save(ctx, manifestPath, manifest); err != nil {
				return storageFailure(err)
			}
			file.State = "completed"
			file.Error = ""
			relative, err := savedRelative(item, entry)
			if err != nil {
				return storageFailure(err)
			}
			file.SavedPath = filepath.Join(job.Destination, relative)
			return s.saveMediaJob(job, emit)
		}
	}
	file.State = "preparing"
	file.Error = ""
	if err := s.saveMediaJob(job, emit); err != nil {
		return storageFailure(err)
	}
	itemDir := filepath.Join(mediaWorkDir(*job), workDirectoryName(item.Key()))
	staging := filepath.Join(itemDir, "staging")
	input := filepath.Join(itemDir, "input.json")
	if err := safefs.Ensure(workBoundary(*job), staging); err != nil {
		return storageFailure(err)
	}
	_, inputErr := os.Stat(input)
	continuing := inputErr == nil && !previouslyCompleted
	if !continuing {
		filtered, err := s.Telegram.FilterExport(data, []domain.MediaItem{item})
		if err != nil {
			return err
		}
		if err := safefs.WriteFile(workBoundary(*job), input, filtered); err != nil {
			return storageFailure(err)
		}
	}
	file.State = "downloading"
	if err := s.saveMediaJob(job, emit); err != nil {
		return storageFailure(err)
	}
	var savedAt time.Time
	var persistErr error
	request := domain.DownloadRequest{JobID: job.ID, Chat: job.Chat, ExportPath: input, Destination: staging, Filter: domain.FilterAllAttachments, Takeout: job.Takeout, Continue: continuing, SkipSame: true}
	err = s.downloadWithRetry(ctx, request, func(event Event) {
		if event.Progress != nil {
			progress := *event.Progress
			progress.ItemKey = item.Key()
			progress.FileName = item.OriginalName
			progress.UpdatedAt = time.Now().UnixMilli()
			progress = mergeFileProgress(file.Progress, progress, item.SizeBytes)
			file.Progress = &progress
			event.Progress = &progress
			if time.Since(savedAt) >= time.Second {
				persistErr = s.Jobs.Save(context.Background(), *job)
				savedAt = time.Now()
			}
		}
		emitEvent(emit, event)
	})
	if err != nil {
		return err
	}
	if persistErr != nil {
		return storageFailure(persistErr)
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	file.State = "finalizing"
	if err := s.saveMediaJob(job, emit); err != nil {
		return storageFailure(err)
	}
	root := job.Destination
	topic := job.Topic
	if topic == nil && item.TopicID > 0 {
		for _, candidate := range job.Chat.Topics {
			if candidate.ID == item.TopicID {
				copy := candidate
				topic = &copy
				break
			}
		}
	}
	if topic != nil {
		dirs, err := domain.BuildChatDirectories(root, fmt.Sprintf("%s [%d]", topic.Title, topic.ID))
		if err != nil {
			return err
		}
		root = dirs.Root
	}
	// Validate size before marking a transfer complete. File existence alone does
	// not prove an interrupted/preallocated file is complete.
	if item.SizeBytes > 0 {
		entries, err := os.ReadDir(staging)
		if err != nil {
			return storageFailure(err)
		}
		valid := false
		for _, entry := range entries {
			if info, err := entry.Info(); err == nil && info.Mode().IsRegular() && info.Size() == item.SizeBytes {
				valid = true
			}
		}
		if !valid {
			return fmt.Errorf("downloaded file is missing or its size does not match Telegram metadata")
		}
	}
	var summary domain.OrganizationSummary
	if organizer, ok := s.Organizer.(ports.ConfinedDownloadOrganizer); ok {
		summary, err = organizer.OrganizeDownloadsWithin(staging, root, job.DestinationRoot, []domain.MediaItem{item}, func(item domain.MediaItem, name string) error {
			relative, err := filepath.Rel(job.Destination, filepath.Join(root, categoryFolder(domain.ClassifyMedia(item.OriginalName)), name))
			if err != nil {
				return err
			}
			manifest.MarkMoving(item, name)
			entry := manifest.Items[item.Key()]
			entry.SavedPath = relative
			manifest.Items[item.Key()] = entry
			return s.Manifests.Save(context.Background(), manifestPath, manifest)
		})
	} else {
		summary, err = s.Organizer.OrganizeDownloads(staging, root, []domain.MediaItem{item})
	}
	if err != nil {
		return storageFailure(err)
	}
	if len(summary.Outcomes) != 1 || summary.Outcomes[0].Status != domain.OrganizationRenamed {
		return fmt.Errorf("requested file was not saved; download output is missing or ambiguous")
	}
	file.SavedPath = summary.Outcomes[0].SavedPath
	relative, err := filepath.Rel(job.Destination, file.SavedPath)
	if err != nil {
		return storageFailure(err)
	}
	manifest.MarkCompleted(item, filepath.Base(file.SavedPath))
	entry := manifest.Items[item.Key()]
	entry.SavedPath = relative
	manifest.Items[item.Key()] = entry
	if err := s.Manifests.Save(context.Background(), manifestPath, manifest); err != nil {
		return storageFailure(err)
	}
	file.State = "completed"
	file.Error = ""
	percent := 100.0
	file.Progress = &domain.ProgressEvent{JobID: job.ID, ItemKey: item.Key(), FileName: item.OriginalName, Percent: &percent, BytesDownloaded: item.SizeBytes, TotalBytes: item.SizeBytes, UpdatedAt: time.Now().UnixMilli()}
	return s.saveMediaJob(job, emit)
}

func mergeFileProgress(previous *domain.ProgressEvent, next domain.ProgressEvent, size int64) domain.ProgressEvent {
	if previous != nil {
		if next.Percent == nil {
			next.Percent = previous.Percent
		}
		if next.BytesDownloaded == 0 {
			next.BytesDownloaded = previous.BytesDownloaded
		}
		if next.TotalBytes == 0 {
			next.TotalBytes = previous.TotalBytes
		}
	}
	if next.TotalBytes == 0 {
		next.TotalBytes = size
	}
	if next.TotalBytes > 0 && next.BytesDownloaded > 0 {
		percent := float64(next.BytesDownloaded) / float64(next.TotalBytes) * 100
		if percent > 100 {
			percent = 100
		}
		next.Percent = &percent
	}
	return next
}

func storageFailure(err error) error {
	return &domain.AppError{Code: domain.ErrorCodeStorage, Message: err.Error(), Cause: err}
}
func (s *Service) stopMedia(ctx context.Context, job domain.DownloadJob, err error, emit func(Event)) (RunResult, error) {
	for i := range job.Files {
		if job.Files[i].State == "completed" || job.Files[i].State == "failed" {
			continue
		}
		if ctx.Err() != nil {
			if errors.Is(context.Cause(ctx), ErrPauseRequested) {
				job.Files[i].State = "paused"
			} else {
				job.Files[i].State = "cancelled"
			}
		} else if job.Files[i].State != "queued" {
			job.Files[i].State = "failed"
			job.Files[i].Error = err.Error()
		}
	}
	return s.fail(ctx, job, err, emit)
}
