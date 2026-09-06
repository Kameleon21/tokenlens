package app

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestTimezonePrecedenceAndLocalCalendar(t *testing.T) {
	t.Setenv("TZ", "")
	now := time.Date(2026, 3, 1, 0, 30, 0, 0, time.UTC)
	calls := 0
	detect := func() (string, error) { calls++; return "America/Los_Angeles", nil }
	o, err := optionsWithLocalTimezone(nil, now, detect)
	if err != nil || o.TZ != "America/Los_Angeles" || o.Range.Since != "2026-02-01" || o.Range.Until != "2026-02-28" || calls != 1 {
		t.Fatalf("system-local month not selected: %+v %v calls=%d", o, err, calls)
	}
	args := strings.Join(backendArgs(o.Range, o.TZ), " ")
	if !strings.Contains(args, "--timezone America/Los_Angeles") || !strings.Contains(args, "--since 2026-02-01") {
		t.Fatal(args)
	}
	t.Setenv("TZ", "Asia/Tokyo")
	o, err = optionsWithLocalTimezone(nil, now, detect)
	if err != nil || o.TZ != "Asia/Tokyo" || o.Range.Since != "2026-03-01" || calls != 1 {
		t.Fatalf("TZ override: %+v %v", o, err)
	}
	t.Setenv("TZ", "invalid/timezone")
	o, err = optionsWithLocalTimezone([]string{"--timezone", "Europe/Dublin"}, now, detect)
	if err != nil || o.TZ != "Europe/Dublin" || calls != 1 {
		t.Fatalf("CLI override: %+v %v", o, err)
	}
}

func TestLocalTimezoneDSTAndCacheIsolation(t *testing.T) {
	t.Setenv("TZ", "")
	for _, stamp := range []string{"2026-03-08T07:30:00Z", "2026-03-09T06:30:00Z", "2026-11-01T07:30:00Z"} {
		now, _ := time.Parse(time.RFC3339, stamp)
		loc, _ := time.LoadLocation("America/Los_Angeles")
		o, err := optionsWithLocalTimezone([]string{"--last", "1"}, now, func() (string, error) { return loc.String(), nil })
		want := now.In(loc).Format("2006-01-02")
		if err != nil || o.Range.Since != want || o.Range.Until != want {
			t.Fatalf("%s: %+v %v", stamp, o, err)
		}
	}
	now := time.Now()
	args := []string{"--since", "20260901", "--until", "20260930", "--cache-dir", t.TempDir()}
	a, err := optionsWithLocalTimezone(args, now, func() (string, error) { return "Europe/Dublin", nil })
	if err != nil {
		t.Fatal(err)
	}
	b, err := optionsWithLocalTimezone(args, now, func() (string, error) { return "America/New_York", nil })
	if err != nil {
		t.Fatal(err)
	}
	ap, _ := snapshotCachePath(a, a.Range)
	bp, _ := snapshotCachePath(b, b.Range)
	if ap == bp {
		t.Fatal("different system timezones share a cache")
	}
}

func TestLocalTimezoneFailureAndVersion(t *testing.T) {
	t.Setenv("TZ", "")
	fail := func() (string, error) { return "", errors.New("timezone unavailable") }
	if _, err := optionsWithLocalTimezone(nil, time.Now(), fail); err == nil {
		t.Fatal("detection failure silently became UTC")
	}
	if o, err := optionsWithLocalTimezone([]string{"--version"}, time.Now(), fail); err != nil || !o.ShowVersion {
		t.Fatal("version requires timezone detection")
	}
	for _, name := range []string{"Local", "bad/timezone"} {
		if _, err := optionsWithLocalTimezone([]string{"--timezone", name}, time.Now(), fail); err == nil {
			t.Fatalf("accepted backend-incompatible timezone %s", name)
		}
	}
}

func TestSystemTimezoneFiles(t *testing.T) {
	for path, want := range map[string]string{
		"/var/db/timezone/zoneinfo/Europe/Dublin":    "Europe/Dublin",
		"/usr/share/zoneinfo/posix/America/New_York": "America/New_York",
		"/usr/share/zoneinfo/Etc/UTC":                "Etc/UTC",
		"/usr/share/zoneinfo/invalid/zone":           "",
		"/etc/localtime":                             "",
	} {
		if got := timezoneFromPath(path); got != want {
			t.Fatalf("%s: %s", path, got)
		}
	}
	dir := t.TempDir()
	config := filepath.Join(dir, "timezone")
	if err := os.WriteFile(config, []byte("Europe/Dublin\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if got := timezoneFromFiles(filepath.Join(dir, "missing"), config); got != "Europe/Dublin" {
		t.Fatal(got)
	}
	// Symlink names take precedence over a stale text configuration.
	target := filepath.Join(dir, "zoneinfo", "Asia", "Tokyo")
	if err := os.MkdirAll(filepath.Dir(target), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "localtime")
	if err := os.Symlink(target, link); err != nil {
		t.Skip("symlink unavailable:", err)
	}
	if got := timezoneFromFiles(link, config); got != "Asia/Tokyo" {
		t.Fatal(got)
	}
}
