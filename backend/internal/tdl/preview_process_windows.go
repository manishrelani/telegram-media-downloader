//go:build windows

package tdl

import (
	"os"
	"os/exec"
)

func configurePreviewProcess(cmd *exec.Cmd)             {}
func interruptPreviewProcess(process *os.Process) error { return process.Kill() }
