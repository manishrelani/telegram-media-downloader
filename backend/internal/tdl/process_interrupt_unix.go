//go:build !windows

package tdl

import (
	"os"
)

// interruptProcess sends SIGINT to tdl so its own shutdown path can persist download
// progress before exiting. The parent process is still bounded by exec.Cmd.WaitDelay.
func interruptProcess(process *os.Process) error {
	return process.Signal(os.Interrupt)
}

func supportsGracefulInterrupt() bool { return true }
