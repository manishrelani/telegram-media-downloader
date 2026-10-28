package domain

import (
	"errors"
	"testing"
)

func TestDownloadJobTransitionReturnsChangedCopy(t *testing.T) {
	original := DownloadJob{ID: "job-1", State: JobQueued}
	next, err := original.Transition(JobPreparing)
	if err != nil {
		t.Fatalf("Transition() error = %v", err)
	}
	if original.State != JobQueued {
		t.Fatalf("original state mutated to %q", original.State)
	}
	if next.State != JobPreparing {
		t.Fatalf("new state = %q, want %q", next.State, JobPreparing)
	}
}

func TestAppErrorSupportsErrorWrapping(t *testing.T) {
	cause := errors.New("permission denied")
	appErr := &AppError{Code: ErrorCodeStorage, Message: "cannot write destination", Retryable: false, Cause: cause}
	if appErr.Error() != "cannot write destination" {
		t.Fatalf("Error() = %q", appErr.Error())
	}
	if !errors.Is(appErr, cause) {
		t.Fatal("AppError should unwrap its cause")
	}
}
