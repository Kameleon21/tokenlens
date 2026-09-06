package app

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

type doctorReport struct {
	out                io.Writer
	warnings, failures int
}

func (d *doctorReport) check(level, name, message string) {
	if level == "WARN" {
		d.warnings++
	}
	if level == "FAIL" {
		d.failures++
	}
	fmt.Fprintf(d.out, "[%s] %s: %s\n", level, name, safe(message))
}

// Probe the nearest existing directory without creating the requested tree.
// This distinguishes a first-run destination from a file or an unwritable path.
func probeDirectory(path string) (string, error) {
	path, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	target := path
	for {
		info, err := os.Stat(target)
		if err == nil {
			if !info.IsDir() {
				return "", fmt.Errorf("%s is a file, not a directory; choose a directory", target)
			}
			f, err := os.CreateTemp(target, ".tokenlens-doctor-*")
			if err != nil {
				return "", fmt.Errorf("cannot write to %s: %w; choose a writable directory or adjust its permissions", target, err)
			}
			closeErr := f.Close()
			removeErr := os.Remove(f.Name())
			if closeErr != nil {
				return "", closeErr
			}
			if removeErr != nil {
				return "", fmt.Errorf("cannot remove probe %s: %w", f.Name(), removeErr)
			}
			if target != path {
				return path + " (not created yet; parent is writable)", nil
			}
			return path + " (writable)", nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
		// Do not mistake a dangling symlink for a directory that can be created.
		if _, linkErr := os.Lstat(target); linkErr == nil {
			return "", fmt.Errorf("%s is a dangling link; repair it or choose another directory", target)
		}
		parent := filepath.Dir(target)
		if parent == target {
			return "", err
		}
		target = parent
	}
}
func (d *doctorReport) directory(name, path string) {
	message, err := probeDirectory(path)
	if err != nil {
		d.check("FAIL", name, err.Error())
		return
	}
	d.check("OK", name, message)
}

func doctorCommand(args []string, out io.Writer) int {
	if len(args) == 1 && (args[0] == "--help" || args[0] == "-h") {
		fmt.Fprintln(out, "Usage: tokenlens doctor [flags]\nUses the same flags and TOML settings as Tokenlens.\nChecks configuration, timezone, export/cache directory access, backend discovery, and local prices.\nNo network requests or backend execution; creates and removes temporary write probes only.\nExit status: 0 = no failures (warnings may remain), 1 = checks failed, 2 = invalid options.")
		return 0
	}
	d := doctorReport{out: out}
	d.check("INFO", "Tokenlens", Version+" · "+runtime.GOOS+"/"+runtime.GOARCH)
	if executable, err := os.Executable(); err == nil {
		d.check("INFO", "Executable", executable)
	}
	if cwd, err := os.Getwd(); err == nil {
		d.check("INFO", "Working directory", cwd)
	}
	d.check("INFO", "Scope", "Local checks only; no network requests or usage collection. Temporary write probes are removed.")
	configFailed := false
	o, err := optionsWithPreferenceReader(args, time.Now(), localTimezone, func(path string) (Preferences, error) {
		prefs, err := readPreferences(path)
		if err != nil {
			configFailed = true
			d.check("FAIL", "Configuration", err.Error())
			d.check("INFO", "Remaining checks", "Using default preferences plus CLI/environment overrides because the configuration is invalid.")
			return defaultPreferences(), nil
		}
		if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
			d.check("OK", "Configuration", path+" (not created yet; using defaults)")
		} else {
			d.check("OK", "Configuration", path)
		}
		return prefs, nil
	})
	if errors.Is(err, flag.ErrHelp) {
		return 0
	}
	optionFailed := err != nil
	if optionFailed {
		d.check("FAIL", "Options", err.Error())
	}
	if o.ShowVersion && !optionFailed {
		return 0
	}
	if validTimezone(o.TZ) {
		d.check("OK", "Timezone", o.TZ)
	} else {
		d.check("FAIL", "Timezone", "Could not resolve "+o.TZ+"; set --timezone to an IANA name such as Europe/Dublin.")
	}
	if o.ExportDir != "" {
		d.directory("Exports", o.ExportDir)
	}
	if o.NoCache {
		d.check("INFO", "Cache", "Snapshot and price/exchange-rate disk caching are disabled by --no-cache.")
	} else if path, err := priceCachePath(o); err != nil {
		d.check("FAIL", "Cache directory", err.Error())
	} else {
		d.directory("Cache directory", filepath.Dir(path))
		d.check("INFO", "Snapshots", fmt.Sprintf("Cache reuse interval: %s; cached usage reports are not scanned.", o.CacheTTL))
	}
	if o.Demo {
		d.check("INFO", "Backend", "Demo uses synthetic data; no backend is needed.")
	} else {
		path, prefix, err := backendCommand(o.Bin)
		if err != nil {
			d.check("FAIL", "Backend", err.Error())
		} else if len(prefix) > 0 {
			d.check("WARN", "Backend", path+" is available as a Bun launcher; ccusage is not installed locally and may need a download. Install the full Tokenlens release bundle for self-contained use.")
		} else {
			d.check("OK", "Backend", path+" (executable found; execution and report compatibility not tested)")
		}
	}
	if hasCCUsageConfig() {
		d.check("WARN", "Pricing configuration", "A ccusage configuration was detected. Tokenlens-managed pricing overrides are disabled; review your ccusage configuration if costs differ.")
	}
	d.prices(o)
	if o.Offline {
		d.check("INFO", "Offline", "Background model-price downloads are disabled. This does not disable currency-rate requests or Bun package downloads.")
	}
	if o.Currency != "USD" {
		d.check("INFO", "Currency", o.Currency+" display reuses exchange rates for 24 hours; connectivity is not tested. Without a saved rate, costs wait for a successful request.")
	}
	fmt.Fprintf(out, "\n%d failure(s), %d warning(s).\n", d.failures, d.warnings)
	if optionFailed {
		return 2
	}
	if configFailed || d.failures > 0 {
		return 1
	}
	return 0
}

func (d *doctorReport) prices(o Options) {
	p, err := decodeCatalog(embeddedPrices)
	if err != nil {
		d.check("FAIL", "Prices", "Bundled catalog is invalid; reinstall an official Tokenlens release.")
		return
	}
	source := "bundled"
	if path, err := priceCachePath(o); err == nil && !o.NoCache {
		f, err := os.Open(path)
		if err == nil {
			data, readErr := io.ReadAll(io.LimitReader(f, maxCatalogBytes+1))
			f.Close()
			cached, decodeErr := decodeCatalog(data)
			if readErr != nil || decodeErr != nil {
				d.check("WARN", "Price cache", path+" is unreadable or invalid; using bundled prices. Refresh prices in the app or remove this cache file.")
			} else if cached.Fetched.After(p.Fetched) {
				p, source = cached, "cached"
			}
		} else if !errors.Is(err, os.ErrNotExist) {
			d.check("WARN", "Price cache", path+" cannot be read; using bundled prices. Check file permissions.")
		}
	}
	loc, _ := time.LoadLocation(o.TZ)
	m := model{o: o, displayLocation: loc}
	level := "OK"
	message := fmt.Sprintf("%s catalog, %d models, fetched %s", source, len(p.Models), m.formatTimestamp(p.Fetched))
	if time.Since(p.Fetched) > priceTTL && o.managedPrices {
		level = "WARN"
		message += "; older than the 6-hour refresh interval. Open Tokenlens without --offline to refresh managed prices."
	}
	if !o.managedPrices {
		message += "; managed price overrides are inactive for this invocation"
	}
	d.check(level, "Prices", message)
}
