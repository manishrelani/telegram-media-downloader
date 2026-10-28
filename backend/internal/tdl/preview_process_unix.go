//go:build !windows

package tdl

import (
	"os"
	"os/exec"
	"syscall"
)

func configurePreviewProcess(cmd *exec.Cmd) { cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true} }
func interruptPreviewProcess(process *os.Process) error {
	return syscall.Kill(-process.Pid, syscall.SIGINT)
}
