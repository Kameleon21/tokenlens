package app

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestExportDirectoryPreferences(t *testing.T) {
	path := isolatedPreferences(t)
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ configured, want string }{
		{"~/Downloads/tokenlens exports", filepath.Join(home, "Downloads", "tokenlens exports")},
		{filepath.Join(t.TempDir(), "out"), ""},
		{"relative-exports", ""},
	} {
		if err := updatePreference(path, func(p *Preferences) { p.ExportDir = tc.configured }); err != nil {
			t.Fatal(err)
		}
		o, err := options(nil, time.Now())
		if err != nil {
			t.Fatal(err)
		}
		want := tc.want
		if want == "" {
			want, err = filepath.Abs(tc.configured)
			if err != nil {
				t.Fatal(err)
			}
		}
		if o.ExportDir != want {
			t.Fatalf("got %s want %s", o.ExportDir, want)
		}
		override := filepath.Join(t.TempDir(), "override")
		o, err = options([]string{"--export-dir", override}, time.Now())
		if err != nil || o.ExportDir != override {
			t.Fatalf("%+v %v", o, err)
		}
		if err := updatePreference(path, func(p *Preferences) { p.Theme = "nord" }); err != nil {
			t.Fatal(err)
		}
		saved, err := readPreferences(path)
		if err != nil || saved.ExportDir != tc.configured {
			t.Fatal("override or preference save changed export_dir")
		}
	}
	for _, invalid := range []string{"", " ", "bad\x00path", "~another/exports"} {
		if _, err := resolveExportDir(invalid); err == nil {
			t.Fatalf("accepted %q", invalid)
		}
	}
	m := fixtureModel()
	m.o.ExportDir = filepath.Join(t.TempDir(), "new", "exports")
	exported, err := m.writeExport("json")
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Dir(exported) != m.o.ExportDir {
		t.Fatal(exported)
	}
}

func TestDoctorReportsMultipleFailuresAndPreservesFiles(t *testing.T) {
	config := isolatedPreferences(t)
	if err := os.MkdirAll(filepath.Dir(config), 0700); err != nil {
		t.Fatal(err)
	}
	original := []byte("unknown_setting = true\n")
	if err := os.WriteFile(config, original, 0600); err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	blocked := filepath.Join(root, "file")
	if err := os.WriteFile(blocked, []byte("preserve"), 0600); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	code := doctorCommand([]string{"--export-dir", blocked, "--cache-dir", filepath.Join(root, "cache"), "--ccusage", filepath.Join(root, "missing-backend")}, &out)
	if code != 1 {
		t.Fatalf("code %d: %s", code, out.String())
	}
	for _, part := range []string{"[FAIL] Configuration", "[FAIL] Exports", "[FAIL] Backend", "[OK] Timezone", "bundled catalog", "Using default preferences"} {
		if !strings.Contains(out.String(), part) {
			t.Fatalf("missing %s: %s", part, out.String())
		}
	}
	data, _ := os.ReadFile(config)
	if !bytes.Equal(data, original) {
		t.Fatal("doctor rewrote invalid config")
	}
	entries, _ := os.ReadDir(root)
	if len(entries) != 1 || entries[0].Name() != "file" {
		t.Fatal("doctor left probes or created directories")
	}
}

func TestDoctorHealthyDemoAndBadPriceCache(t *testing.T) {
	isolatedPreferences(t)
	root := t.TempDir()
	var out bytes.Buffer
	args := []string{"--demo", "--export-dir", filepath.Join(root, "exports"), "--cache-dir", root}
	if code := doctorCommand(args, &out); code != 0 {
		t.Fatalf("%d: %s", code, out.String())
	}
	if !strings.Contains(out.String(), "not created yet") {
		t.Fatal(out.String())
	}
	if err := os.WriteFile(filepath.Join(root, "prices-v1.json"), []byte("broken"), 0600); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if code := doctorCommand(args, &out); code != 0 || !strings.Contains(out.String(), "[WARN] Price cache") {
		t.Fatalf("%d: %s", code, out.String())
	}
	out.Reset()
	args = append(args, "--no-cache")
	if code := doctorCommand(args, &out); code != 0 || strings.Contains(out.String(), "[WARN] Price cache") {
		t.Fatalf("%d: %s", code, out.String())
	}
	out.Reset()
	if code := doctorCommand([]string{"--help"}, &out); code != 0 || !strings.Contains(out.String(), "Usage: tokenlens doctor") {
		t.Fatal(out.String())
	}
}

func TestDoctorInvalidTimezoneAndBlockedParent(t *testing.T) {
	isolatedPreferences(t)
	root := t.TempDir()
	var out bytes.Buffer
	if code := doctorCommand([]string{"--demo", "--timezone", "not/a/zone", "--export-dir", root, "--cache-dir", root}, &out); code != 2 || !strings.Contains(out.String(), "[FAIL] Timezone") {
		t.Fatalf("%d %s", code, out.String())
	}
	file := filepath.Join(root, "file")
	if err := os.WriteFile(file, nil, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := probeDirectory(filepath.Join(file, "child")); err == nil {
		t.Fatal("accepted file as parent")
	}
}
