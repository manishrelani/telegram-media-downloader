package application

import (
	"example.com/telegram-media-downloader/backend/internal/domain"
	"example.com/telegram-media-downloader/backend/internal/safefs"
	"fmt"
	"path/filepath"
	"strings"
)

func savedRelative(item domain.MediaItem, entry domain.ManifestEntry) (string, error) {
	name := entry.SavedName
	if name == "" || name == "." || name == ".." || strings.ContainsAny(name, "/\\\x00") || filepath.IsAbs(name) {
		return "", fmt.Errorf("invalid manifest filename")
	}
	expected := filepath.Ext(domain.FilenameFor("", item.OriginalName))
	if !strings.EqualFold(filepath.Ext(name), expected) {
		return "", fmt.Errorf("saved file type does not match the downloaded attachment")
	}
	rel := filepath.Join(categoryFolder(domain.ClassifyMedia(item.OriginalName)), name)
	if entry.SavedPath != "" {
		rel = filepath.FromSlash(strings.ReplaceAll(entry.SavedPath, "\\", "/"))
	}
	if !filepath.IsLocal(rel) || filepath.Base(rel) != name {
		return "", fmt.Errorf("invalid manifest relative path")
	}
	return rel, nil
}

// ValidateSavedDownload is used at the OS-opener boundary, independently of the
// earlier manifest check. Persisted completion alone is not authority to open a path.
func ValidateSavedDownload(job domain.DownloadJob, file domain.FileTask) (string, error) {
	if file.State != "completed" || !filepath.IsAbs(file.SavedPath) {
		return "", fmt.Errorf("not a saved download")
	}
	rel, err := safefs.Relative(job.Destination, file.SavedPath)
	if err != nil {
		return "", err
	}
	if !strings.EqualFold(filepath.Ext(rel), filepath.Ext(domain.FilenameFor("", file.Item.OriginalName))) {
		return "", fmt.Errorf("saved file type mismatch")
	}
	allowed := job.DestinationRoot
	if allowed == "" {
		allowed = filepath.Dir(job.Destination)
	}
	r, rel, err := safefs.Open(allowed, file.SavedPath)
	if err != nil {
		return "", err
	}
	defer r.Close()
	f, err := r.Open(rel)
	if err != nil {
		return "", err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("saved download is not a regular file")
	}
	return filepath.Clean(file.SavedPath), nil
}
