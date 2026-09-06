package app

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// No app test should depend on or write the developer's saved preferences.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "tokenlens-test-config-")
	if err != nil {
		panic(err)
	}
	os.Setenv("XDG_CONFIG_HOME", dir)
	os.Setenv("APPDATA", dir)
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

func isolatedPreferences(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("APPDATA", dir)
	t.Setenv("TOKENLENS_CURRENCY", "")
	t.Setenv("TZ", "UTC")
	path, err := configPath()
	if err != nil {
		t.Fatal(err)
	}
	return path
}

func TestConfigLocations(t *testing.T) {
	home := filepath.Join(t.TempDir(), "home")
	xdg := filepath.Join(t.TempDir(), "custom")
	for _, tc := range []struct{ os, xdg, appdata, want string }{
		{"darwin", "", "", filepath.Join(home, ".config")},
		{"linux", "", "", filepath.Join(home, ".config")},
		{"darwin", xdg, "", xdg},
		{"linux", xdg, "", xdg},
		{"darwin", "relative", "", filepath.Join(home, ".config")},
		{"windows", xdg, home, home},
	} {
		t.Run(tc.os+tc.xdg, func(t *testing.T) {
			got, err := configPathFor(tc.os, func(k string) string {
				if k == "XDG_CONFIG_HOME" {
					return tc.xdg
				}
				return tc.appdata
			}, func() (string, error) { return home, nil })
			if err != nil || got != filepath.Join(tc.want, "tokenlens", "config.toml") {
				t.Fatalf("%q %v", got, err)
			}
		})
	}
	if _, err := configPathFor("windows", func(string) string { return "" }, func() (string, error) { return home, nil }); err == nil {
		t.Fatal("missing APPDATA accepted")
	}
}

func TestPreferencePrecedenceAndNoStartupWrites(t *testing.T) {
	path := isolatedPreferences(t)
	if _, err := options(nil, time.Now()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("startup created config")
	}
	if err := updatePreference(path, func(p *Preferences) {
		p.Currency = "EUR"
		p.Theme = "nord"
		p.Grouping = "weekly"
		p.Display = "tokens"
		p.CompactNumbers = true
		p.Layout = "stacked"
	}); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(path)
	o, err := options(nil, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	m := newModel(context.Background(), o)
	if o.Currency != "EUR" || o.Theme != "nord" || o.Group != "weekly" || m.cost || !m.compactNumbers || m.layout != 1 {
		t.Fatalf("saved settings not loaded: %+v", o)
	}
	t.Setenv("TOKENLENS_CURRENCY", "GBP")
	o, err = options(nil, time.Now())
	if err != nil || o.Currency != "GBP" {
		t.Fatalf("env precedence: %+v %v", o, err)
	}
	o, err = options([]string{"monthly", "--currency", "JPY", "--theme", "light"}, time.Now())
	if err != nil || o.Currency != "JPY" || o.Theme != "light" || o.Group != "monthly" {
		t.Fatalf("flag precedence: %+v %v", o, err)
	}
	after, _ := os.ReadFile(path)
	if !bytes.Equal(before, after) {
		t.Fatal("one-off overrides persisted")
	}
	// Saving a different preference must not persist the runtime overrides.
	m = newModel(context.Background(), o)
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	_ = updated
	saved, err := readPreferences(path)
	if err != nil || saved.Currency != "EUR" || saved.Theme != "nord" || saved.Grouping != "weekly" {
		t.Fatalf("unrelated preferences overwritten: %+v %v", saved, err)
	}
}

func TestTUIChoicesPersistAndRestore(t *testing.T) {
	path := isolatedPreferences(t)
	o, err := options(nil, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	m := newModel(context.Background(), o)
	for _, key := range []string{"e", "w", "c", "n", "v"} {
		next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)})
		m = next.(model)
	}
	o, err = options(nil, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	restored := newModel(context.Background(), o)
	if restored.o.Currency != "EUR" || restored.o.Group != "weekly" || restored.cost || !restored.compactNumbers || restored.layout != 1 {
		t.Fatalf("choices not restored: %+v", restored.o)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	// Windows does not implement Unix permission bits.
	if os.PathSeparator != '\\' && info.Mode().Perm() != 0600 {
		t.Fatalf("config mode: %v", info.Mode())
	}
}

func TestThemePreviewAndCancelNeverSave(t *testing.T) {
	path := isolatedPreferences(t)
	o, err := options(nil, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	m := newModel(context.Background(), o)
	m.openThemePicker()
	next, _ := m.updateThemePicker(tea.KeyMsg{Type: tea.KeyDown})
	m = next.(model)
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("preview saved")
	}
	next, _ = m.updateThemePicker(tea.KeyMsg{Type: tea.KeyEsc})
	m = next.(model)
	if m.o.Theme != "dark" {
		t.Fatal("cancel did not restore theme")
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("cancel saved")
	}
	m.openThemePicker()
	next, _ = m.updateThemePicker(tea.KeyMsg{Type: tea.KeyDown})
	m = next.(model)
	next, _ = m.updateThemePicker(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(model)
	saved, err := readPreferences(path)
	if err != nil || saved.Theme != m.o.Theme || saved.Theme == "dark" {
		t.Fatalf("applied theme not saved: %+v %v", saved, err)
	}
}

func TestInvalidConfigAndResetRecovery(t *testing.T) {
	path := isolatedPreferences(t)
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{"theme = [", `theme = "unknown"`, `currency = "EU"`, `grouping = "yearly"`, `display = "other"`, `layout = "other"`, `compact_numbers = "true"`, `typo = true`} {
		if err := os.WriteFile(path, []byte(bad), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := options(nil, time.Now()); err == nil || !strings.Contains(err.Error(), path) {
			t.Fatalf("bad config accepted: %q %v", bad, err)
		}
		if err := updatePreference(path, func(p *Preferences) { p.Currency = "EUR" }); err == nil {
			t.Fatal("invalid config overwritten")
		}
		after, _ := os.ReadFile(path)
		if string(after) != bad {
			t.Fatal("invalid config was damaged")
		}
	}
	if _, err := options([]string{"--version"}, time.Now()); err != nil {
		t.Fatal(err)
	}
	if _, err := options([]string{"--help"}, time.Now()); !errors.Is(err, flag.ErrHelp) {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := configCommand([]string{"path"}, &out); err != nil || strings.TrimSpace(out.String()) != path {
		t.Fatalf("path command: %s %v", out.String(), err)
	}
	for i := 0; i < 2; i++ {
		if err := configCommand([]string{"reset"}, &out); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := options(nil, time.Now()); err != nil {
		t.Fatal(err)
	}
}

func TestSaveFailureKeepsTUIUsable(t *testing.T) {
	dir := t.TempDir()
	blocker := filepath.Join(dir, "file")
	if err := os.WriteFile(blocker, []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	m := newModel(context.Background(), Options{configPath: filepath.Join(blocker, "config.toml")})
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	m = next.(model)
	if !m.compactNumbers || !strings.Contains(m.notice, "Could not save") {
		t.Fatalf("save failure handling: %q", m.notice)
	}
	data, _ := os.ReadFile(blocker)
	if string(data) != "keep" {
		t.Fatal("existing file damaged")
	}
}
