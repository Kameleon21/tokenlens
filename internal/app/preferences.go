package app

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"slices"

	"github.com/pelletier/go-toml/v2"
)

// Preferences contains only choices deliberately remembered between sessions.
type Preferences struct {
	DateFormat     string `toml:"date_format"`
	ClockFormat    string `toml:"clock_format"`
	SessionsSort   string `toml:"sessions_sort"`
	ModelsSort     string `toml:"models_sort"`
	Currency       string `toml:"currency"`
	Theme          string `toml:"theme"`
	Grouping       string `toml:"grouping"`
	Display        string `toml:"display"`
	CompactNumbers bool   `toml:"compact_numbers"`
	Layout         string `toml:"layout"`
}

func defaultPreferences() Preferences {
	return Preferences{DateFormat: "european", ClockFormat: "24h", SessionsSort: "cost_desc", ModelsSort: "cost_desc", Currency: "USD", Theme: "dark", Grouping: "daily", Display: "cost", Layout: "dashboard"}
}

func configPath() (string, error) {
	return configPathFor(runtime.GOOS, os.Getenv, os.UserHomeDir)
}

func configPathFor(goos string, getenv func(string) string, home func() (string, error)) (string, error) {
	var base string
	if goos == "windows" {
		base = getenv("APPDATA")
		if base == "" {
			return "", fmt.Errorf("APPDATA is not set")
		}
	} else {
		base = getenv("XDG_CONFIG_HOME")
		if base == "" || !filepath.IsAbs(base) {
			dir, err := home()
			if err != nil {
				return "", err
			}
			base = filepath.Join(dir, ".config")
		}
	}
	return filepath.Join(base, "tokenlens", "config.toml"), nil
}

func (p Preferences) validate() error {
	if !slices.Contains([]string{"european", "us", "iso"}, p.DateFormat) {
		return fmt.Errorf("date_format must be european, us, or iso")
	}
	if !slices.Contains([]string{"12h", "24h"}, p.ClockFormat) {
		return fmt.Errorf("clock_format must be 12h or 24h")
	}
	if !slices.Contains(sessionSorts, p.SessionsSort) {
		return fmt.Errorf("invalid sessions_sort %q", p.SessionsSort)
	}
	if !slices.Contains(modelSorts, p.ModelsSort) {
		return fmt.Errorf("invalid models_sort %q", p.ModelsSort)
	}
	if _, err := currencyCode(p.Currency); err != nil {
		return err
	}
	if !slices.Contains(themeNames, p.Theme) {
		return fmt.Errorf("unknown theme %q", p.Theme)
	}
	if !slices.Contains([]string{"daily", "weekly", "monthly"}, p.Grouping) {
		return fmt.Errorf("grouping must be daily, weekly, or monthly")
	}
	if !slices.Contains([]string{"cost", "tokens"}, p.Display) {
		return fmt.Errorf("display must be cost or tokens")
	}
	if !slices.Contains([]string{"dashboard", "stacked"}, p.Layout) {
		return fmt.Errorf("layout must be dashboard or stacked")
	}
	return nil
}

func readPreferences(path string) (Preferences, error) {
	p := defaultPreferences()
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return p, nil
	}
	if err == nil {
		err = toml.NewDecoder(bytes.NewReader(data)).DisallowUnknownFields().Decode(&p)
	}
	if err == nil {
		err = p.validate()
	}
	if err != nil {
		return p, fmt.Errorf("preferences %s: %w; edit the file or run tokenlens config reset", path, err)
	}
	p.Currency, _ = currencyCode(p.Currency)
	return p, nil
}

// Re-read before each change so a CLI override or another session's unrelated
// preference is not accidentally persisted. Replace atomically to avoid torn files.
func updatePreference(path string, change func(*Preferences)) error {
	p, err := readPreferences(path)
	if err != nil {
		return err
	}
	change(&p)
	if err = p.validate(); err != nil {
		return err
	}
	data, err := toml.Marshal(p)
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".config-*.toml")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err = f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}

func (m *model) savePreference(change func(*Preferences)) {
	// Models constructed without runtime options (e.g. tests) never touch user files.
	if m.o.configPath == "" {
		return
	}
	if err := updatePreference(m.o.configPath, change); err != nil {
		m.notice = "Could not save preferences: " + err.Error()
	}
}

func configCommand(args []string, out io.Writer) error {
	if len(args) != 1 || (args[0] != "path" && args[0] != "reset") {
		return fmt.Errorf("usage: tokenlens config path|reset")
	}
	path, err := configPath()
	if err != nil {
		return err
	}
	if args[0] == "path" {
		_, err = fmt.Fprintln(out, path)
		return err
	}
	if err = os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	_, err = fmt.Fprintln(out, "Saved preferences reset. Existing sessions keep their current settings.")
	return err
}
