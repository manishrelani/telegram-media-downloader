//go:build windows

package tdl

import "os"

// Windows cannot deliver Unix SIGINT semantics to an arbitrary child process using
// os.Process.Signal. We use a hard stop for cancellation and disable Pause in the UI
// until a dedicated Windows console-control implementation is added.
func interruptProcess(process *os.Process) error {
	return process.Kill()
}

func supportsGracefulInterrupt() bool { return false }
