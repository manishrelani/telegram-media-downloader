package domain

import "testing"

func percentPointer(value float64) *float64 { return &value }

func TestSummarizeProgress(t *testing.T) {
	tests := []struct {
		name      string
		items     []ItemProgress
		wantTotal int
		wantDone  int
		wantFail  int
		wantSkip  int
		wantWork  *float64
		wantAll   *float64
	}{
		{name: "empty has unknown percentages", wantTotal: 0},
		{
			name:      "completed and queued",
			items:     []ItemProgress{{State: ItemCompleted}, {State: ItemQueued}},
			wantTotal: 2, wantDone: 1, wantWork: percentPointer(50), wantAll: percentPointer(50),
		},
		{
			name:      "failed resolved but not counted as transferred",
			items:     []ItemProgress{{State: ItemCompleted}, {State: ItemFailed}},
			wantTotal: 2, wantDone: 1, wantFail: 1, wantWork: percentPointer(100), wantAll: percentPointer(50),
		},
		{
			name:      "unknown active percentage makes overall unknown",
			items:     []ItemProgress{{State: ItemDownloading}},
			wantTotal: 1, wantAll: nil, wantWork: percentPointer(0),
		},
		{
			name:      "known active percentage",
			items:     []ItemProgress{{State: ItemDownloading, Percent: percentPointer(35)}, {State: ItemQueued}},
			wantTotal: 2, wantAll: percentPointer(17.5), wantWork: percentPointer(0),
		},
		{
			name:      "percentage is clamped",
			items:     []ItemProgress{{State: ItemDownloading, Percent: percentPointer(140)}},
			wantTotal: 1, wantAll: percentPointer(100), wantWork: percentPointer(0),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SummarizeProgress(tt.items)
			if got.TotalItems != tt.wantTotal || got.CompletedItems != tt.wantDone || got.FailedItems != tt.wantFail || got.SkippedItems != tt.wantSkip {
				t.Fatalf("counts = %+v; expected total=%d complete=%d failed=%d skipped=%d", got, tt.wantTotal, tt.wantDone, tt.wantFail, tt.wantSkip)
			}
			assertPercent(t, "resolved", got.ResolvedPercent, tt.wantWork)
			assertPercent(t, "overall", got.OverallPercent, tt.wantAll)
		})
	}
}

func assertPercent(t *testing.T, name string, got, want *float64) {
	t.Helper()
	if want == nil {
		if got != nil {
			t.Fatalf("%s percent = %v, want unknown", name, *got)
		}
		return
	}
	if got == nil {
		t.Fatalf("%s percent is unknown, want %v", name, *want)
	}
	if *got-*want > 0.001 || *want-*got > 0.001 {
		t.Fatalf("%s percent = %v, want %v", name, *got, *want)
	}
}
