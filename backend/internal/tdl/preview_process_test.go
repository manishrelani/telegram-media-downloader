//go:build !windows

package tdl

import (
	"context"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"testing"
	"time"
)

func TestPreviewOwnedProcess(t *testing.T) {
	mode := os.Getenv("TMM_TEST_PREVIEW_PROCESS")
	if mode == "" {
		return
	}
	marker := os.Getenv("TMM_TEST_PREVIEW_MARKER")
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt)
	if mode == "worker" {
		os.WriteFile(marker, []byte("running"), 0600)
		<-signals
		os.Remove(marker)
		os.Exit(0)
	}
	child := exec.Command(os.Args[0], "-test.run=^TestPreviewOwnedProcess$")
	child.Env = append(os.Environ(), "TMM_TEST_PREVIEW_PROCESS=worker")
	if child.Start() != nil {
		os.Exit(2)
	}
	child.Wait()
	os.Exit(0)
}
func TestPreviewCancellationStopsChildAndAllowsCleanup(t *testing.T) {
	for _, streaming := range []bool{false, true} {
		t.Run(map[bool]string{false: "Run", true: "RunStream"}[streaming], func(t *testing.T) {
			marker := filepath.Join(t.TempDir(), "child-alive")
			t.Setenv("TMM_TEST_PREVIEW_PROCESS", "parent")
			t.Setenv("TMM_TEST_PREVIEW_MARKER", marker)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			done := make(chan error, 1)
			go func() {
				executor := OSExecutor{Binary: os.Args[0], PreviewProcessGroup: true}
				var err error
				if streaming {
					err = executor.RunStream(ctx, []string{"-test.run=^TestPreviewOwnedProcess$"}, func(string, string) {})
				} else {
					_, _, err = executor.Run(ctx, "-test.run=^TestPreviewOwnedProcess$")
				}
				done <- err
			}()
			deadline := time.Now().Add(2 * time.Second)
			for {
				if _, err := os.Stat(marker); err == nil {
					break
				}
				if time.Now().After(deadline) {
					t.Fatal("preview child did not start")
				}
				time.Sleep(10 * time.Millisecond)
			}
			cancel()
			select {
			case <-done:
			case <-time.After(3 * time.Second):
				t.Fatal("preview did not stop promptly")
			}
			if _, err := os.Stat(marker); !os.IsNotExist(err) {
				t.Fatal("preview child cleanup did not run")
			}
		})
	}
}
