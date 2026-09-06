package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Shell expansion does not happen inside TOML. Support the current user's
// home prefix explicitly; relative paths retain the CLI's working-directory semantics.
func resolveExportDir(path string) (string, error) {
	if strings.TrimSpace(path) == "" || strings.ContainsRune(path, 0) {
		return "", fmt.Errorf("export directory must be a nonempty path without NUL characters")
	}
	if path == "~" || strings.HasPrefix(path, "~/") || strings.HasPrefix(path, "~\\") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve export directory: %w", err)
		}
		if len(path) > 1 {
			path = filepath.Join(home, path[2:])
		} else {
			path = home
		}
	} else if strings.HasPrefix(path, "~") {
		return "", fmt.Errorf("export directory: use ~/ for your home or an absolute path; ~user is not supported")
	}
	return filepath.Abs(path)
}
