//go:build darwin || linux

package app

import (
	"os/exec"
	"syscall"
	"time"
)

// Cancel the whole backend process group, including Bun's installer child.
func configureBackend(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
	cmd.WaitDelay = 3 * time.Second
}
