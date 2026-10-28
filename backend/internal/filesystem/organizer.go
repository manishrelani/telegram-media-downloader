// Package filesystem contains side-effecting filesystem adapters. Domain naming rules are
// delegated to the pure domain package; this package only matches, creates and moves files.
package filesystem

import (
	"errors"
	"example.com/telegram-media-downloader/backend/internal/safefs"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"example.com/telegram-media-downloader/backend/internal/domain"
)

type ItemStatus = domain.OrganizationStatus

const (
	ItemRenamed          = domain.OrganizationRenamed
	ItemSkipped          = domain.OrganizationSkipped
	ItemFailed           = domain.OrganizationFailed
	ItemAlreadyCompleted = domain.OrganizationAlreadyCompleted
)

type ItemOutcome = domain.OrganizationOutcome
type OrganizationSummary = domain.OrganizationSummary

type stagedFile struct {
	name string
	path string
}

// OrganizeDownloads moves tdl's message-ID-prefixed files from a staging directory into
// Videos, Documents or Attachments, applying the Telegram message caption as the filename.
// Missing/ambiguous input is reported per item. Storage failures stop finalization
// immediately and return both the partial summary and an actionable error.
func OrganizeDownloads(stagingDir, chatRoot string, items []domain.MediaItem) (OrganizationSummary, error) {
	return OrganizeDownloadsWithHook(stagingDir, chatRoot, items, nil)
}

// OrganizeDownloadsWithHook persists the planned output name before moving each file.
// The hook must return successfully before any filesystem side effect occurs for that item.
func OrganizeDownloadsWithHook(stagingDir, chatRoot string, items []domain.MediaItem, beforeMove func(domain.MediaItem, string) error) (OrganizationSummary, error) {
	return OrganizeDownloadsWithin(stagingDir, chatRoot, filepath.Dir(chatRoot), items, beforeMove)
}

func OrganizeDownloadsWithin(stagingDir, chatRoot, allowedRoot string, items []domain.MediaItem, beforeMove func(domain.MediaItem, string) error) (OrganizationSummary, error) {
	stagingDir = strings.TrimSpace(stagingDir)
	chatRoot = strings.TrimSpace(chatRoot)
	if stagingDir == "" || chatRoot == "" {
		return OrganizationSummary{}, errors.New("staging directory and chat destination are required")
	}
	boundary, _, err := safefs.Open(allowedRoot, chatRoot)
	if err != nil {
		return OrganizationSummary{}, organizationStorageError(err)
	}
	defer boundary.Close()
	sourceRoot, err := os.OpenRoot(stagingDir)
	if err != nil {
		return OrganizationSummary{}, organizationStorageError(err)
	}
	defer sourceRoot.Close()
	info, err := os.Stat(stagingDir)
	if err != nil {
		return OrganizationSummary{}, organizationStorageError(fmt.Errorf("open download staging directory: %w", err))
	}
	if !info.IsDir() {
		return OrganizationSummary{}, organizationStorageError(fmt.Errorf("staging path is not a directory: %s", stagingDir))
	}

	folders := map[domain.MediaCategory]string{
		domain.CategoryVideo:    filepath.Join(chatRoot, "Videos"),
		domain.CategoryDocument: filepath.Join(chatRoot, "Documents"),
		domain.CategoryOther:    filepath.Join(chatRoot, "Attachments"),
	}
	for _, dir := range folders {
		rel, _ := safefs.Relative(allowedRoot, dir)
		if err := safefs.Check(boundary, rel); err != nil {
			return OrganizationSummary{}, organizationStorageError(err)
		}
		if err := boundary.MkdirAll(rel, 0o700); err != nil {
			return OrganizationSummary{}, organizationStorageError(fmt.Errorf("create output directory %q: %w", dir, err))
		}
	}

	sourceDir, err := sourceRoot.Open(".")
	if err != nil {
		return OrganizationSummary{}, organizationStorageError(err)
	}
	defer sourceDir.Close()
	entries, err := sourceDir.ReadDir(-1)
	if err != nil {
		return OrganizationSummary{}, organizationStorageError(fmt.Errorf("read staging directory: %w", err))
	}
	staged := make([]stagedFile, 0, len(entries))
	for _, entry := range entries {
		if entry.Type()&os.ModeSymlink != 0 {
			return OrganizationSummary{}, organizationStorageError(fmt.Errorf("symlink in staging directory"))
		}
		if entry.IsDir() {
			continue
		}
		staged = append(staged, stagedFile{name: entry.Name(), path: filepath.Join(stagingDir, entry.Name())})
	}
	sort.Slice(staged, func(i, j int) bool { return staged[i].name < staged[j].name })

	allocators := make(map[domain.MediaCategory]*domain.FilenameAllocator, len(folders))
	for category, dir := range folders {
		rel, _ := safefs.Relative(allowedRoot, dir)
		opened, err := boundary.Open(rel)
		if err != nil {
			return OrganizationSummary{}, organizationStorageError(err)
		}
		entries, err := opened.ReadDir(-1)
		opened.Close()
		names := make([]string, 0, len(entries))
		for _, entry := range entries {
			names = append(names, entry.Name())
		}
		if err != nil {
			return OrganizationSummary{}, organizationStorageError(fmt.Errorf("list output directory %q: %w", dir, err))
		}
		allocators[category] = domain.NewFilenameAllocator(names)
	}

	summary := OrganizationSummary{Outcomes: make([]ItemOutcome, 0, len(items))}
	for _, item := range items {
		outcome := ItemOutcome{ChatID: item.ChatID, MessageID: item.MessageID, OriginalName: item.OriginalName}
		source, matchStatus := findStagedFile(staged, item)
		if matchStatus == "missing" {
			outcome.Status = ItemSkipped
			outcome.Reason = "downloaded file was not found in the staging directory"
			summary.Skipped++
			summary.Outcomes = append(summary.Outcomes, outcome)
			continue
		}
		if matchStatus == "ambiguous" {
			outcome.Status = ItemFailed
			outcome.Reason = "multiple staged files matched this message; refusing to choose one automatically"
			summary.Failed++
			summary.Outcomes = append(summary.Outcomes, outcome)
			continue
		}

		category := domain.ClassifyMedia(item.OriginalName)
		destinationDir := folders[category]
		allocator := allocators[category]
		finalName := allocator.Allocate(item.Caption, item.OriginalName)
		finalPath := filepath.Join(destinationDir, finalName)
		// Avoid silent replacement if a file appeared after the initial scan.
		for attempt := 0; attempt < 100; attempt++ {
			if _, statErr := boundary.Lstat(mustRelative(allowedRoot, finalPath)); errors.Is(statErr, os.ErrNotExist) {
				break
			} else if statErr != nil {
				return organizationStorageFailure(summary, outcome, fmt.Errorf("inspect destination: %w", statErr))
			}
			finalName = allocator.Allocate(item.Caption, item.OriginalName)
			finalPath = filepath.Join(destinationDir, finalName)
		}
		if _, statErr := boundary.Lstat(mustRelative(allowedRoot, finalPath)); statErr == nil {
			return organizationStorageFailure(summary, outcome, fmt.Errorf("could not allocate a collision-free destination filename"))
		} else if !errors.Is(statErr, os.ErrNotExist) {
			return organizationStorageFailure(summary, outcome, fmt.Errorf("inspect destination: %w", statErr))
		}
		sourceInfo, statErr := sourceRoot.Stat(filepath.Base(source.path))
		if statErr != nil {
			return organizationStorageFailure(summary, outcome, statErr)
		}
		if item.SizeBytes <= 0 {
			item.SizeBytes = sourceInfo.Size()
		}
		if beforeMove != nil {
			if err := beforeMove(item, finalName); err != nil {
				return organizationStorageFailure(summary, outcome, fmt.Errorf("persist pending file move: %w", err))
			}
		}
		if err := safefs.Check(boundary, mustRelative(allowedRoot, destinationDir)); err != nil {
			return organizationStorageFailure(summary, outcome, err)
		}
		if err := copyRootNoReplace(sourceRoot, filepath.Base(source.path), boundary, mustRelative(allowedRoot, finalPath)); err != nil {
			return organizationStorageFailure(summary, outcome, fmt.Errorf("save downloaded file: %w", err))
		}
		outcome.Status = ItemRenamed
		outcome.SavedPath = finalPath
		summary.Renamed++
		summary.Outcomes = append(summary.Outcomes, outcome)
	}
	return summary, nil
}

func organizationStorageFailure(summary OrganizationSummary, outcome ItemOutcome, cause error) (OrganizationSummary, error) {
	outcome.Status = ItemFailed
	outcome.Reason = cause.Error()
	summary.Failed++
	summary.Outcomes = append(summary.Outcomes, outcome)
	return summary, organizationStorageError(cause)
}

func organizationStorageError(cause error) error {
	return &domain.AppError{Code: domain.ErrorCodeStorage, Message: cause.Error(), Cause: cause}
}

func findStagedFile(files []stagedFile, item domain.MediaItem) (stagedFile, string) {
	prefix := strconv.FormatInt(item.ChatID, 10) + "_" + strconv.FormatInt(item.MessageID, 10) + "_"
	expected := prefix + domain.FilenameFor("", item.OriginalName)
	var exact []stagedFile
	var candidates []stagedFile
	for _, file := range files {
		if strings.EqualFold(file.name, expected) {
			exact = append(exact, file)
		}
		if strings.HasPrefix(strings.ToLower(file.name), strings.ToLower(prefix)) {
			candidates = append(candidates, file)
		}
	}
	if len(exact) == 1 {
		return exact[0], "found"
	}
	if len(exact) > 1 || len(candidates) > 1 {
		return stagedFile{}, "ambiguous"
	}
	if len(candidates) == 1 {
		return candidates[0], "found"
	}
	return stagedFile{}, "missing"
}

func listNames(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	return names, nil
}

func copyRootNoReplace(source *os.Root, sourceName string, destination *os.Root, target string) error {
	input, err := source.Open(sourceName)
	if err != nil {
		return err
	}
	defer input.Close()
	output, err := destination.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("reserve destination without overwriting: %w", err)
	}
	original, err := output.Stat()
	if err != nil {
		output.Close()
		return err
	}
	cleanup := func() {
		current, err := destination.Lstat(target)
		if err == nil && os.SameFile(original, current) {
			_ = destination.Remove(target)
		}
	}
	_, copyErr := io.Copy(output, input)
	syncErr := output.Sync()
	closeErr := output.Close()
	if err := errors.Join(copyErr, syncErr, closeErr); err != nil {
		cleanup()
		return fmt.Errorf("save downloaded file: %w", err)
	}
	current, err := destination.Lstat(target)
	if err != nil || !os.SameFile(original, current) {
		cleanup()
		return fmt.Errorf("destination changed while saving; staging retained")
	}
	if err := source.Remove(sourceName); err != nil {
		cleanup()
		return fmt.Errorf("remove staging file: %w", err)
	}
	return nil
}

// DownloadOrganizer adapts the organizer function to the application-facing port.
type DownloadOrganizer struct{}

func (DownloadOrganizer) OrganizeDownloads(stagingDir, chatRoot string, items []domain.MediaItem) (domain.OrganizationSummary, error) {
	return OrganizeDownloads(stagingDir, chatRoot, items)
}

func (DownloadOrganizer) OrganizeDownloadsWithHook(stagingDir, chatRoot string, items []domain.MediaItem, beforeMove func(domain.MediaItem, string) error) (domain.OrganizationSummary, error) {
	return OrganizeDownloadsWithHook(stagingDir, chatRoot, items, beforeMove)
}

func mustRelative(root, path string) string { rel, _ := safefs.Relative(root, path); return rel }
func (DownloadOrganizer) OrganizeDownloadsWithin(staging, output, allowed string, items []domain.MediaItem, hook func(domain.MediaItem, string) error) (domain.OrganizationSummary, error) {
	return OrganizeDownloadsWithin(staging, output, allowed, items, hook)
}
