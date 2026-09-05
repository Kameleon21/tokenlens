//go:build !darwin && !linux

package app

import (
	"os/exec"
	"time"
)

func configureBackend(cmd *exec.Cmd) { cmd.WaitDelay = 3 * time.Second }
