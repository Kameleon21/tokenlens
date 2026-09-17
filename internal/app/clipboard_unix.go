//go:build darwin || linux

package app

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

// Use cancellable native commands; clipboard.WriteAll does not support contexts.
func writeClipboard(ctx context.Context, text string) error {
	var args []string
	if runtime.GOOS == "darwin" {
		args = []string{"pbcopy"}
	} else {
		if os.Getenv("WAYLAND_DISPLAY") != "" {
			if _, err := exec.LookPath("wl-copy"); err == nil {
				args = []string{"wl-copy"}
			}
		}
		if len(args) == 0 {
			for _, candidate := range [][]string{{"xclip", "-selection", "clipboard"}, {"xsel", "--clipboard", "--input"}} {
				if _, err := exec.LookPath(candidate[0]); err == nil {
					args = candidate
					break
				}
			}
		}
		if len(args) == 0 {
			return fmt.Errorf("install wl-clipboard (Wayland) or xclip/xsel (X11) and run in a desktop session")
		}
	}
	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	cmd.Stdin = strings.NewReader(text)
	cmd.WaitDelay = time.Second
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("%s unavailable: check local clipboard access (%w)", args[0], err)
	}
	return nil
}
