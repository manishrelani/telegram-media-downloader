package tdl

import (
	"errors"
	"os/exec"
	"strings"
	"testing"

	"example.com/telegram-media-downloader/backend/internal/domain"
)

func TestCommandErrorClassifiesDatabaseLock(t *testing.T) {
	got := commandError("tdl chat ls", []byte("Current database is used by another process"), errors.New("exit status 1"))
	var appErr *domain.AppError
	if !errors.As(got, &appErr) {
		t.Fatalf("error type = %T, want *domain.AppError", got)
	}
	if appErr.Code != domain.ErrorCodeStorage {
		t.Fatalf("code = %q", appErr.Code)
	}
	if !strings.Contains(got.Error(), "database is used by another process") {
		t.Fatalf("lost actionable detail: %v", got)
	}
}

func TestCommandErrorClassifiesFloodWait(t *testing.T) {
	got := commandError("tdl download", []byte("FLOOD_WAIT_90"), errors.New("exit status 1"))
	var appErr *domain.AppError
	if !errors.As(got, &appErr) {
		t.Fatalf("error type = %T", got)
	}
	if appErr.Code != domain.ErrorCodeRateLimited || !appErr.Retryable {
		t.Fatalf("error = %+v", appErr)
	}
}

func TestCommandErrorClassifiesAuthenticationError(t *testing.T) {
	got := commandError("tdl chat ls", []byte("AUTH_KEY_UNREGISTERED"), errors.New("exit status 1"))
	var appErr *domain.AppError
	if !errors.As(got, &appErr) {
		t.Fatalf("error type = %T", got)
	}
	if appErr.Code != domain.ErrorCodeUnauthorized || appErr.Retryable {
		t.Fatalf("error = %+v", appErr)
	}
}

func TestCommandErrorClassifiesMissingExecutable(t *testing.T) {
	got := commandError("tdl version", nil, &exec.Error{Name: "tdl", Err: exec.ErrNotFound})
	var appErr *domain.AppError
	if !errors.As(got, &appErr) {
		t.Fatalf("error type = %T", got)
	}
	if appErr.Code != domain.ErrorCodeUnavailable {
		t.Fatalf("code = %q", appErr.Code)
	}
	if !strings.Contains(got.Error(), "bundled runtime") {
		t.Fatalf("not actionable: %v", got)
	}
}
