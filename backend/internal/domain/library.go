package domain

import (
	"fmt"
	"path"
	"strings"
)

type Topic struct {
	ID    int64  `json:"id"`
	Title string `json:"title"`
}

// Limit is the number of recent attachments, before local file-type filtering.
// Zero explicitly means the complete history of the selected scope.
type MediaQuery struct {
	Chat    Chat   `json:"chat"`
	Topic   *Topic `json:"topic,omitempty"`
	Limit   int    `json:"limit"`
	Refresh bool   `json:"refresh,omitempty"`
}

type MediaSelection struct {
	Topic       *Topic   `json:"topic,omitempty"`
	FileTypes   []string `json:"fileTypes"`
	SnapshotID  string   `json:"snapshotId,omitempty"`
	Keys        []string `json:"keys,omitempty"`
	EntireScope bool     `json:"entireScope,omitempty"`
}

type FileTask struct {
	Availability string         `json:"availability,omitempty"`
	Item         MediaItem      `json:"item"`
	State        string         `json:"state"`
	Progress     *ProgressEvent `json:"progress,omitempty"`
	SavedPath    string         `json:"savedPath,omitempty"`
	Error        string         `json:"error,omitempty"`
}

var fileTypeExtensions = map[string]map[string]struct{}{
	"video":   videoExtensions,
	"image":   stringSet(".jpg", ".jpeg", ".png", ".webp", ".gif", ".heic", ".heif", ".avif", ".bmp", ".tif", ".tiff"),
	"audio":   stringSet(".mp3", ".m4a", ".aac", ".ogg", ".opus", ".wav", ".flac", ".alac", ".aiff", ".amr", ".wma"),
	"pdf":     stringSet(".pdf"),
	"office":  stringSet(".doc", ".docx", ".xls", ".xlsx", ".ppt", ".pptx", ".odt", ".ods", ".odp", ".pages", ".numbers", ".key", ".rtf"),
	"text":    stringSet(".txt", ".md", ".csv", ".tsv", ".log"),
	"ebook":   stringSet(".epub", ".mobi", ".azw", ".azw3", ".djvu"),
	"archive": stringSet(".zip", ".rar", ".7z", ".tar", ".gz", ".bz2", ".xz", ".tgz", ".zst"),
	"code":    stringSet(".go", ".py", ".js", ".ts", ".tsx", ".jsx", ".dart", ".java", ".c", ".h", ".cpp", ".cs", ".rs", ".swift", ".kt", ".html", ".css", ".scss", ".json", ".yaml", ".yml", ".xml", ".sql", ".sh", ".ipynb", ".tex"),
}

func FileType(name string) string {
	ext := strings.ToLower(path.Ext(strings.ReplaceAll(name, "\\", "/")))
	for _, kind := range []string{"code", "video", "image", "audio", "pdf", "office", "text", "ebook", "archive"} {
		if _, ok := fileTypeExtensions[kind][ext]; ok {
			return kind
		}
	}
	return "other"
}

func ValidateFileTypes(types []string) error {
	if len(types) == 0 {
		return fmt.Errorf("select at least one file type")
	}
	for _, kind := range types {
		if kind != "all" && kind != "other" {
			if _, ok := fileTypeExtensions[kind]; !ok {
				return fmt.Errorf("unknown file type %q", kind)
			}
		}
	}
	return nil
}

func MatchesFileTypes(name string, types []string) bool {
	kind := FileType(name)
	for _, selected := range types {
		if selected == "all" || selected == kind {
			return true
		}
	}
	return false
}
