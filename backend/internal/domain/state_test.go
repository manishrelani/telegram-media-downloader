package domain

import "testing"

func TestJobStateTransitions(t *testing.T) {
	tests := []struct {
		from JobState
		to   JobState
		want bool
	}{
		{JobQueued, JobPreparing, true},
		{JobPreparing, JobDownloading, true},
		{JobDownloading, JobPausing, true},
		{JobPausing, JobPaused, true},
		{JobPaused, JobPreparing, true},
		{JobDownloading, JobCancelling, true},
		{JobCancelling, JobCancelled, true},
		{JobDownloading, JobCompleted, true},
		{JobDownloading, JobFailed, true},
		{JobFailed, JobQueued, true},
		{JobCompleted, JobPaused, false},
		{JobQueued, JobCompleted, false},
		{JobCancelled, JobDownloading, false},
		{JobDownloading, JobQueued, false},
		{JobPaused, JobDownloading, false},
		{JobCompleted, JobCompleted, false},
	}
	for _, tt := range tests {
		t.Run(string(tt.from)+"_to_"+string(tt.to), func(t *testing.T) {
			if got := CanTransition(tt.from, tt.to); got != tt.want {
				t.Fatalf("CanTransition(%q, %q) = %v, want %v", tt.from, tt.to, got, tt.want)
			}
		})
	}
}

func TestTransitionReturnsErrorForInvalidTransition(t *testing.T) {
	got, err := Transition(JobCompleted, JobPaused)
	if err == nil {
		t.Fatal("Transition() error = nil, want invalid transition error")
	}
	if got != JobCompleted {
		t.Fatalf("Transition() state = %q, want original state %q", got, JobCompleted)
	}
}

func TestTransitionAllowsRetryFromFailed(t *testing.T) {
	got, err := Transition(JobFailed, JobQueued)
	if err != nil {
		t.Fatalf("Transition() error = %v", err)
	}
	if got != JobQueued {
		t.Fatalf("Transition() = %q, want %q", got, JobQueued)
	}
}
