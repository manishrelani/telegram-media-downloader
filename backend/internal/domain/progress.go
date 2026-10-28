package domain

// ItemState represents the state of one media item in a job-level progress summary.
type ItemState string

const (
	ItemQueued      ItemState = "queued"
	ItemPreparing   ItemState = "preparing"
	ItemDownloading ItemState = "downloading"
	ItemCompleted   ItemState = "completed"
	ItemFailed      ItemState = "failed"
	ItemSkipped     ItemState = "skipped"
)

type ItemProgress struct {
	State   ItemState `json:"state"`
	Percent *float64  `json:"percent,omitempty"`
}

type ProgressSummary struct {
	TotalItems      int      `json:"totalItems"`
	CompletedItems  int      `json:"completedItems"`
	FailedItems     int      `json:"failedItems"`
	SkippedItems    int      `json:"skippedItems"`
	QueuedItems     int      `json:"queuedItems"`
	ActiveItems     int      `json:"activeItems"`
	ResolvedPercent *float64 `json:"resolvedPercent,omitempty"`
	OverallPercent  *float64 `json:"overallPercent,omitempty"`
}

// SummarizeProgress keeps two meanings distinct: ResolvedPercent is the fraction of items
// that reached any terminal state, while OverallPercent estimates successful transfer
// progress. OverallPercent is unknown if an active transfer provides no percentage.
func SummarizeProgress(items []ItemProgress) ProgressSummary {
	s := ProgressSummary{TotalItems: len(items)}
	if len(items) == 0 {
		return s
	}

	var completedTransfer float64
	activeProgressKnown := true
	resolved := 0
	for _, item := range items {
		switch item.State {
		case ItemQueued:
			s.QueuedItems++
		case ItemPreparing:
			s.ActiveItems++
			activeProgressKnown = false
		case ItemDownloading:
			s.ActiveItems++
			if item.Percent == nil {
				activeProgressKnown = false
				continue
			}
			completedTransfer += clampPercent(*item.Percent) / 100
		case ItemCompleted:
			s.CompletedItems++
			resolved++
			completedTransfer++
		case ItemFailed:
			s.FailedItems++
			resolved++
		case ItemSkipped:
			s.SkippedItems++
			resolved++
		default:
			// Unknown states are counted as active/unknown rather than silently claiming
			// completion. This is defensive for state files from a newer version.
			s.ActiveItems++
			activeProgressKnown = false
		}
	}

	resolvedPercent := float64(resolved) / float64(s.TotalItems) * 100
	s.ResolvedPercent = &resolvedPercent
	if activeProgressKnown {
		overall := completedTransfer / float64(s.TotalItems) * 100
		s.OverallPercent = &overall
	}
	return s
}

func clampPercent(value float64) float64 {
	if value < 0 {
		return 0
	}
	if value > 100 {
		return 100
	}
	return value
}
