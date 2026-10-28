package progress

import "testing"

func TestParseANSIAndPartialTransferUpdates(t *testing.T) {
	got, ok := Parse("\x1b[2K\x1b[32mLesson\x1b[0m 6.2% [1.50 MB in 1s; ~ETA: 2m; 1.50 MB/s\x1b[0m")
	if !ok || !got.HasPercent || got.Percent != 6.2 || got.Label != "Lesson" {
		t.Fatalf("snapshot=%+v", got)
	}
	partial, ok := Parse("[2.00 MB in 2s; 1.00 MB/s")
	if !ok || partial.HasPercent || partial.Transferred != "2.00 MB" {
		t.Fatalf("partial=%+v", partial)
	}
}

func TestParseUserObservedTdlProgress(t *testing.T) {
	line := "Namaste Node.js(4~ ... 35.1% [#...] [98.00 MB in 49.682s; ~ETA: 1m32s; 1.97 MB/s **CPU: 0.31% Memory: 41.89 MB Goroutines: 84**"
	got, ok := Parse(line)
	if !ok {
		t.Fatal("expected recognizable progress")
	}
	if !got.HasPercent || got.Percent != 35.1 {
		t.Fatalf("percent = %#v", got)
	}
	if got.Label != "Namaste Node.js(4~ ..." {
		t.Fatalf("label = %q, want leading media label without progress/metrics", got.Label)
	}
	if got.Transferred != "98.00 MB" {
		t.Fatalf("transferred = %q", got.Transferred)
	}
	if got.ETA != "1m32s" {
		t.Fatalf("ETA = %q", got.ETA)
	}
	if got.Speed != "1.97 MB/s" {
		t.Fatalf("speed = %q", got.Speed)
	}
}

func TestParseRejectsMetricsOnlyLine(t *testing.T) {
	if _, ok := Parse("CPU: 0.31% Memory: 41.89 MB Goroutines: 84"); ok {
		t.Fatal("metrics-only line should not become file progress")
	}
}

func TestParseUnknownProgressDoesNotInventValues(t *testing.T) {
	got, ok := Parse("Downloading media now")
	if ok {
		t.Fatal("expected unrecognized line")
	}
	if got.HasPercent {
		t.Fatal("parser must not invent a percentage")
	}
}
