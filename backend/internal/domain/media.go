package domain

import (
	"fmt"
	"path"
	"sort"
	"strings"
)

type MediaCategory string

const (
	CategoryVideo    MediaCategory = "video"
	CategoryDocument MediaCategory = "document"
	CategoryOther    MediaCategory = "other"
)

type FilterMode string

const (
	FilterVideosAndDocuments FilterMode = "videos_and_documents"
	FilterVideosOnly         FilterMode = "videos_only"
	FilterDocumentsOnly      FilterMode = "documents_only"
	FilterAllAttachments     FilterMode = "all_attachments"
)

var videoExtensions = stringSet(
	".mp4", ".mov", ".mkv", ".m4v", ".webm", ".avi", ".3gp", ".mpeg", ".mpg", ".m2v", ".ts", ".mts", ".m2ts", ".ogv", ".wmv", ".flv",
)

var documentExtensions = stringSet(
	".pdf", ".doc", ".docx", ".ppt", ".pptx", ".xls", ".xlsx", ".txt", ".md", ".csv", ".rtf", ".odt", ".ods", ".odp", ".epub", ".pages", ".numbers", ".key", ".tex",
)

func ClassifyMedia(filename string) MediaCategory {
	ext := strings.ToLower(path.Ext(strings.ReplaceAll(filename, "\\", "/")))
	if _, ok := videoExtensions[ext]; ok {
		return CategoryVideo
	}
	if _, ok := documentExtensions[ext]; ok {
		return CategoryDocument
	}
	return CategoryOther
}

func ShouldIncludeMedia(filename string, mode FilterMode) (bool, error) {
	switch mode {
	case FilterVideosAndDocuments:
		category := ClassifyMedia(filename)
		return category == CategoryVideo || category == CategoryDocument, nil
	case FilterVideosOnly:
		return ClassifyMedia(filename) == CategoryVideo, nil
	case FilterDocumentsOnly:
		return ClassifyMedia(filename) == CategoryDocument, nil
	case FilterAllAttachments:
		return true, nil
	default:
		return false, fmt.Errorf("unsupported media filter mode %q", mode)
	}
}

func stringSet(values ...string) map[string]struct{} {
	result := make(map[string]struct{}, len(values))
	for _, value := range values {
		result[value] = struct{}{}
	}
	return result
}

// ExtensionsForFilter returns a deterministic allowlist suitable for tdl's -i flag.
// The returned extensions omit the leading dot. All-attachments mode intentionally
// returns nil so the downloader does not exclude unknown/less-common file extensions.
func ExtensionsForFilter(mode FilterMode) ([]string, error) {
	var selected map[string]struct{}
	switch mode {
	case FilterVideosAndDocuments:
		selected = make(map[string]struct{}, len(videoExtensions)+len(documentExtensions))
		for ext := range videoExtensions {
			selected[ext] = struct{}{}
		}
		for ext := range documentExtensions {
			selected[ext] = struct{}{}
		}
	case FilterVideosOnly:
		selected = videoExtensions
	case FilterDocumentsOnly:
		selected = documentExtensions
	case FilterAllAttachments:
		return nil, nil
	default:
		return nil, fmt.Errorf("unsupported media filter mode %q", mode)
	}
	result := make([]string, 0, len(selected))
	for ext := range selected {
		result = append(result, strings.TrimPrefix(ext, "."))
	}
	sort.Strings(result)
	return result, nil
}

func (mode FilterMode) Valid() bool {
	switch mode {
	case FilterVideosAndDocuments, FilterVideosOnly, FilterDocumentsOnly, FilterAllAttachments:
		return true
	default:
		return false
	}
}
