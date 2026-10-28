package domain

import (
	"path"
	"strings"
	"unicode"
)

const maxFilenameBaseBytes = 180

// FilenameFor chooses Telegram's message caption when it is non-empty and otherwise
// falls back to the exported source filename. The source file extension is preserved.
// Unsafe filename characters are replaced because downloaded captions are untrusted data.
func FilenameFor(caption, originalFilename string) string {
	base, ext := splitFilename(originalFilename)
	caption = CleanText(caption)

	if caption == "" {
		caption = base
	} else if ext != "" && strings.EqualFold(path.Ext(caption), ext) {
		// Avoid a caption such as "notes.pdf" becoming "notes.pdf.pdf".
		caption = strings.TrimSuffix(caption, path.Ext(caption))
	}

	name := sanitizeFilenameBase(caption)
	if name == "" {
		name = sanitizeFilenameBase(base)
	}
	if name == "" {
		name = "download"
	}
	if isWindowsReservedName(name) {
		name = "_" + name
	}
	return name + ext
}

// FilenameAllocator reserves names case-insensitively because common macOS and Windows
// filesystems are case-insensitive by default. Pass existing filenames excluding a source
// file that is itself about to be renamed.
type FilenameAllocator struct {
	used map[string]struct{}
}

func NewFilenameAllocator(existingNames []string) *FilenameAllocator {
	a := &FilenameAllocator{used: make(map[string]struct{}, len(existingNames))}
	for _, name := range existingNames {
		a.used[normalizeCollisionKey(name)] = struct{}{}
	}
	return a
}

func (a *FilenameAllocator) Allocate(caption, originalFilename string) string {
	if a == nil {
		return FilenameFor(caption, originalFilename)
	}
	candidate := FilenameFor(caption, originalFilename)
	if a.reserveIfFree(candidate) {
		return candidate
	}

	base, ext := splitFilename(candidate)
	for suffix := 2; ; suffix++ {
		candidate = truncateBase(base, maxFilenameBaseBytes-len([]byte(" ("+itoa(suffix)+")"))) + " (" + itoa(suffix) + ")" + ext
		if a.reserveIfFree(candidate) {
			return candidate
		}
	}
}

func (a *FilenameAllocator) reserveIfFree(name string) bool {
	key := normalizeCollisionKey(name)
	if _, exists := a.used[key]; exists {
		return false
	}
	a.used[key] = struct{}{}
	return true
}

func splitFilename(filename string) (base, ext string) {
	// Exported filenames should be path components, but tolerate paths from other OSes.
	filename = strings.ReplaceAll(filename, "\\", "/")
	filename = path.Base(filename)
	if filename == "." || filename == "/" {
		return "", ""
	}
	ext = path.Ext(filename)
	base = strings.TrimSuffix(filename, ext)
	return base, ext
}

func sanitizeFilenameBase(value string) string {
	var b strings.Builder
	b.Grow(len(value))
	for _, r := range value {
		if unicode.IsControl(r) {
			b.WriteRune('-')
			continue
		}
		switch r {
		case '<', '>', ':', '"', '/', '\\', '|', '?', '*':
			b.WriteRune('-')
		default:
			b.WriteRune(r)
		}
	}
	value = CleanText(b.String())
	value = strings.Trim(value, " .")
	value = truncateBase(value, maxFilenameBaseBytes)
	return strings.Trim(value, " .")
}

func truncateBase(value string, maxBytes int) string {
	if maxBytes <= 0 {
		return ""
	}
	if len([]byte(value)) <= maxBytes {
		return strings.TrimRight(value, " .")
	}
	var b strings.Builder
	for _, r := range value {
		if b.Len()+len(string(r)) > maxBytes {
			break
		}
		b.WriteRune(r)
	}
	return strings.TrimRight(b.String(), " .")
}

func isWindowsReservedName(name string) bool {
	upper := strings.ToUpper(strings.TrimSpace(name))
	first := strings.SplitN(upper, ".", 2)[0]
	switch first {
	case "CON", "PRN", "AUX", "NUL", "CLOCK$":
		return true
	}
	if len(first) == 4 {
		prefix := first[:3]
		n := first[3]
		if n >= '1' && n <= '9' && (prefix == "COM" || prefix == "LPT") {
			return true
		}
	}
	return false
}

func normalizeCollisionKey(filename string) string {
	return strings.ToLower(strings.TrimSpace(filename))
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
