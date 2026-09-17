//go:build darwin || linux

package app

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestClipboardCommand(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PATH", dir)
	t.Setenv("WAYLAND_DISPLAY", "test-wayland")
	name := "pbcopy"
	if runtime.GOOS == "linux" {
		name = "wl-copy"
	}
	path := filepath.Join(dir, name)
	target := filepath.Join(dir, "copied")
	t.Setenv("COPY_TEST_OUTPUT", target)
	if err := os.WriteFile(path, []byte("#!/bin/sh\n/bin/cat > \"$COPY_TEST_OUTPUT\"\n"), 0755); err != nil {
		t.Fatal(err)
	}
	text := "Unicode 日本語 🦎 literal $(command) `command`"
	if err := writeClipboard(context.Background(), text); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(target)
	if err != nil || string(got) != text {
		t.Fatalf("got %q, err %v", got, err)
	}
	if err := os.WriteFile(path, []byte("#!/bin/sh\nexit 1\n"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := writeClipboard(context.Background(), text); err == nil {
		t.Fatal("ignored command failure")
	}
	if err := os.WriteFile(path, []byte("#!/bin/sh\nexec /bin/sleep 10\n"), 0755); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if err := writeClipboard(ctx, text); err != context.DeadlineExceeded {
		t.Fatalf("timeout: %v", err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := writeClipboard(context.Background(), text); err == nil {
		t.Fatal("missing utility reported success")
	}
}
