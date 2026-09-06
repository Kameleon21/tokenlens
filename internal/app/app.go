package app

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"github.com/Kameleon21/tokenlens/internal/datefilter"
	tea "github.com/charmbracelet/bubbletea"
	"math"
	"os"
	"slices"
	"strings"
	"time"
)

type Options struct {
	managedPrices                             bool
	priceRevision                             string
	preferences                               Preferences
	configPath                                string
	ShowVersion                               bool
	CacheTTL                                  time.Duration
	Offline                                   bool
	CacheDir                                  string
	NoCache                                   bool
	Theme, ExportDir, PlanAgent, PlanCurrency string
	PlanCost                                  float64
	BillingDay                                int
	Group, TZ, Bin                            string
	Currency                                  string
	Range                                     datefilter.Range
	Demo                                      bool
}

func options(args []string, now time.Time) (Options, error) {
	return optionsWithLocalTimezone(args, now, localTimezone)
}

func optionsWithLocalTimezone(args []string, now time.Time, detect func() (string, error)) (Options, error) {
	return optionsWithPreferenceReader(args, now, detect, readPreferences)
}

func optionsWithPreferenceReader(args []string, now time.Time, detect func() (string, error), read func(string) (Preferences, error)) (Options, error) {
	o := Options{Group: "daily", Bin: "ccusage", Currency: "USD"}
	explicitGroup := false
	if c := os.Getenv("TOKENLENS_CURRENCY"); c != "" {
		o.Currency = c
	}
	if tz := os.Getenv("TZ"); tz != "" {
		o.TZ = tz
	}
	if len(args) > 0 && (args[0] == "daily" || args[0] == "weekly" || args[0] == "monthly") {
		explicitGroup = true
		o.Group = args[0]
		args = args[1:]
	}
	f := flag.NewFlagSet("tokenlens", flag.ContinueOnError)
	f.BoolVar(&o.ShowVersion, "version", false, "print the Tokenlens version and exit")
	var since, until string
	var last int
	f.StringVar(&since, "since", "", "inclusive start: YYYY-MM-DD or YYYYMMDD")
	f.StringVar(&until, "until", "", "inclusive end: YYYY-MM-DD or YYYYMMDD")
	f.IntVar(&last, "last", 0, "last N periods including current; cannot combine with date bounds")
	f.StringVar(&o.TZ, "timezone", o.TZ, "IANA timezone (default system timezone; TZ overrides the system default)")
	f.StringVar(&o.Bin, "ccusage", "ccusage", "ccusage executable path")
	f.StringVar(&o.CacheDir, "cache-dir", "", "snapshot cache directory (default OS user cache / tokenlens)")
	f.BoolVar(&o.Offline, "offline", false, "disable background price downloads (use local prices)")
	f.DurationVar(&o.CacheTTL, "cache-ttl", 5*time.Minute, "reuse recent reports for this long; 0 always reloads; r forces refresh")
	f.BoolVar(&o.NoCache, "no-cache", false, "disable usage snapshots and disk price/exchange-rate caches")
	f.StringVar(&o.Theme, "theme", "dark", strings.Join(themeNames, ", "))
	f.StringVar(&o.ExportDir, "export-dir", "exports", "directory for filtered CSV/JSON/SVG/PNG exports")
	f.Float64Var(&o.PlanCost, "plan-cost", 0, "configured monthly plan price in startup display currency")
	f.StringVar(&o.PlanAgent, "plan-agent", "", "agent covered by your plan, e.g. claude")
	f.IntVar(&o.BillingDay, "billing-day", 1, "monthly billing start day (1–31, clamped in shorter months)")
	f.StringVar(&o.Currency, "currency", o.Currency, "display currency (e.g. EUR); default TOKENLENS_CURRENCY, saved preference, or USD; fetches ECB reference rate")
	f.BoolVar(&o.Demo, "demo", false, "clearly labeled synthetic demo; no backend needed")
	f.Usage = func() {
		fmt.Fprint(f.Output(), "Tokenlens — a local lens on agent usage\n\nUsage: tokenlens [daily|weekly|monthly] [flags]\n       tokenlens config path|reset\n       tokenlens doctor [flags]\n\nNo range flags: current calendar month. Costs originate in USD; --currency converts display amounts.\n")
		f.PrintDefaults()
	}
	if e := f.Parse(args); e != nil {
		return o, e
	}
	if f.NArg() > 0 {
		return o, fmt.Errorf("unexpected argument %q", f.Arg(0))
	}
	if o.ShowVersion {
		return o, nil
	}
	path, err := configPath()
	if err != nil {
		return o, err
	}
	prefs, err := read(path)
	if err != nil {
		return o, err
	}
	o.preferences, o.configPath = prefs, path
	provided := map[string]bool{}
	f.Visit(func(v *flag.Flag) { provided[v.Name] = true })
	if !provided["currency"] && os.Getenv("TOKENLENS_CURRENCY") == "" {
		o.Currency = prefs.Currency
	}
	if !provided["export-dir"] {
		o.ExportDir = prefs.ExportDir
	}
	o.ExportDir, err = resolveExportDir(o.ExportDir)
	if err != nil {
		return o, err
	}
	if !provided["theme"] {
		o.Theme = prefs.Theme
	}
	if !explicitGroup {
		o.Group = prefs.Grouping
	}
	hasLast := false
	f.Visit(func(v *flag.Flag) {
		if v.Name == "last" {
			hasLast = true
		}
	})
	if hasLast && last == 0 {
		return o, fmt.Errorf("--last must be between 1 and 10000")
	}
	if o.CacheTTL < 0 {
		return o, fmt.Errorf("--cache-ttl must not be negative")
	}
	if !slices.Contains(themeNames, o.Theme) {
		return o, fmt.Errorf("--theme must be one of: %s", strings.Join(themeNames, ", "))
	}
	if o.PlanCost < 0 || math.IsNaN(o.PlanCost) || math.IsInf(o.PlanCost, 0) {
		return o, fmt.Errorf("--plan-cost must be a finite nonnegative amount")
	}
	if o.BillingDay < 1 || o.BillingDay > 31 {
		return o, fmt.Errorf("--billing-day must be 1–31")
	}
	currency, e := currencyCode(o.Currency)
	if e != nil {
		return o, e
	}
	o.Currency = currency
	o.PlanCurrency = currency
	if o.TZ == "" {
		o.TZ, e = detect()
		if e != nil {
			return o, e
		}
	}
	if !validTimezone(o.TZ) {
		return o, fmt.Errorf("invalid IANA timezone %q", o.TZ)
	}
	loc, e := time.LoadLocation(o.TZ)
	if e != nil {
		return o, fmt.Errorf("invalid timezone %q", o.TZ)
	}
	o.Range, e = datefilter.Resolve(since, until, last, o.Group, now, loc)
	o.managedPrices = !o.Demo && !provided["ccusage"] && !hasCCUsageConfig()
	return o, e
}

// Run starts the terminal app and returns a process exit code.
func Run(args []string) int {
	if len(args) > 0 && args[0] == "doctor" {
		return doctorCommand(args[1:], os.Stdout)
	}
	if len(args) > 0 && args[0] == "config" {
		if err := configCommand(args[1:], os.Stdout); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 2
		}
		return 0
	}
	o, e := options(args, time.Now())
	if errors.Is(e, flag.ErrHelp) {
		return 0
	}
	if e != nil {
		fmt.Fprintln(os.Stderr, e)
		return 2
	}
	if o.ShowVersion {
		fmt.Fprintln(os.Stdout, "tokenlens "+Version)
		return 0
	}
	applyTheme(o.Theme)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if _, e = tea.NewProgram(newModel(ctx, o), tea.WithAltScreen(), tea.WithMouseAllMotion()).Run(); e != nil {
		fmt.Fprintln(os.Stderr, e)
		return 1
	}
	return 0
}
