package app

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Use an IANA name rather than "Local" or a fixed UTC offset: ccusage needs the
// same timezone rules as our calendar calculations, including historical DST.
func validTimezone(name string) bool {
	if name == "" || name == "Local" {
		return false
	}
	_, err := time.LoadLocation(name)
	return err == nil
}

func timezoneFromPath(path string) string {
	path = filepath.ToSlash(path)
	if _, name, ok := strings.Cut(path, "/zoneinfo/"); ok {
		name = strings.TrimPrefix(strings.TrimPrefix(name, "posix/"), "right/")
		if validTimezone(name) {
			return name
		}
	}
	return ""
}

func timezoneFromFiles(localtimePath, timezonePath string) string {
	if path, err := filepath.EvalSymlinks(localtimePath); err == nil {
		if name := timezoneFromPath(path); name != "" {
			return name
		}
	}
	if data, err := os.ReadFile(timezonePath); err == nil {
		if name := strings.TrimSpace(string(data)); validTimezone(name) {
			return name
		}
	}
	return ""
}

func localTimezone() (string, error) {
	// macOS and most Unix systems expose the IANA name through these files.
	if name := timezoneFromFiles("/etc/localtime", "/etc/timezone"); name != "" {
		return name, nil
	}
	// On Windows, or systems with a copied localtime file, the JS runtime used
	// by ccusage can resolve the OS timezone. This is local and never downloads.
	for _, runtime := range []string{"node", "bun"} {
		path, err := exec.LookPath(runtime)
		if err != nil {
			continue
		}
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		cmd := exec.CommandContext(ctx, path, "-e", "process.stdout.write(Intl.DateTimeFormat().resolvedOptions().timeZone)")
		for _, entry := range os.Environ() {
			if !strings.HasPrefix(entry, "TZ=") {
				cmd.Env = append(cmd.Env, entry)
			}
		}
		out, err := cmd.Output()
		cancel()
		if name := strings.TrimSpace(string(out)); err == nil && validTimezone(name) {
			return name, nil
		}
	}
	if name := time.Local.String(); validTimezone(name) {
		return name, nil
	}
	return "", fmt.Errorf("could not determine the system timezone; set --timezone or TZ to an IANA name such as Europe/Dublin")
}
