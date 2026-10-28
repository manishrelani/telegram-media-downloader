package progress

import (
	"regexp"
	"strconv"
	"strings"
)

// Snapshot is deliberately partial: the CLI does not provide a stable machine-readable
// progress protocol, so fields are only populated when the currently supported output yields them.
type Snapshot struct {
	Label       string  `json:"label,omitempty"`
	Percent     float64 `json:"percent"`
	HasPercent  bool    `json:"hasPercent"`
	Transferred string  `json:"transferred,omitempty"`
	Elapsed     string  `json:"elapsed,omitempty"`
	ETA         string  `json:"eta,omitempty"`
	Speed       string  `json:"speed,omitempty"`
	Raw         string  `json:"raw,omitempty"`
}

var percentRE = regexp.MustCompile(`(?:^|\s)(\d{1,3}(?:\.\d+)?)%`)
var transferRE = regexp.MustCompile(`\[([0-9]+(?:\.[0-9]+)?\s+[KMGT]?i?B)\s+in\s+([0-9]+(?:\.[0-9]+)?s)`)
var etaRE = regexp.MustCompile(`(?:~?ETA:\s*)([^;\]\r\n]+)`)
var speedRE = regexp.MustCompile(`([0-9]+(?:\.[0-9]+)?\s+[KMGT]?i?B/s)`)
var ansiRE = regexp.MustCompile(`\x1b\[[0-?]*[ -/]*[@-~]`)

func Parse(text string) (Snapshot, bool) {
	clean := strings.TrimSpace(ansiRE.ReplaceAllString(strings.TrimRight(text, "\r\n"), ""))
	if clean == "" {
		return Snapshot{}, false
	}
	// tdl's progress display may append CPU/RAM/goroutine metrics after the transfer status.
	// Keep Raw for diagnosis, but extract only the fields the UI can confidently identify.
	s := Snapshot{Raw: clean}
	progressPart := clean
	// Ignore process metrics appended after the transfer status; their CPU percentage
	// must never be mistaken for the current file's download percentage.
	for _, marker := range []string{"CPU:", "Memory:", "Goroutines:"} {
		if i := strings.Index(progressPart, marker); i >= 0 {
			progressPart = progressPart[:i]
		}
	}
	if loc := percentRE.FindStringSubmatchIndex(progressPart); len(loc) >= 4 {
		label := strings.TrimSpace(progressPart[:loc[0]])
		label = strings.Trim(label, "|/\\-—·…*#[]() \t")
		if len(label) > 120 {
			label = strings.TrimSpace(label[len(label)-120:])
		}
		s.Label = label
		if v, err := strconv.ParseFloat(progressPart[loc[2]:loc[3]], 64); err == nil && v >= 0 && v <= 100 {
			s.Percent, s.HasPercent = v, true
		}
	}
	if m := transferRE.FindStringSubmatch(progressPart); len(m) == 3 {
		s.Transferred, s.Elapsed = m[1], m[2]
	}
	if m := etaRE.FindStringSubmatch(progressPart); len(m) == 2 {
		s.ETA = strings.TrimSpace(m[1])
	}
	if m := speedRE.FindStringSubmatch(progressPart); len(m) == 2 {
		s.Speed = m[1]
	}
	if !s.HasPercent && s.Transferred == "" && s.ETA == "" && s.Speed == "" {
		return s, false
	}
	return s, true
}
