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
	"time"
)

type Options struct {
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
	o := Options{Group: "daily", TZ: "UTC", Bin: "ccusage", Currency: "USD"}
	if c := os.Getenv("TOKENLENS_CURRENCY"); c != "" {
		o.Currency = c
	}
	if tz := os.Getenv("TZ"); tz != "" {
		o.TZ = tz
	}
	if len(args) > 0 && (args[0] == "daily" || args[0] == "weekly" || args[0] == "monthly") {
		o.Group = args[0]
		args = args[1:]
	}
	f := flag.NewFlagSet("tokenlens", flag.ContinueOnError)
	var since, until string
	var last int
	f.StringVar(&since, "since", "", "inclusive start: YYYY-MM-DD or YYYYMMDD")
	f.StringVar(&until, "until", "", "inclusive end: YYYY-MM-DD or YYYYMMDD")
	f.IntVar(&last, "last", 0, "last N periods including current; cannot combine with date bounds")
	f.StringVar(&o.TZ, "timezone", o.TZ, "IANA timezone (default UTC, or TZ environment variable)")
	f.StringVar(&o.Bin, "ccusage", "ccusage", "ccusage executable path")
	f.StringVar(&o.CacheDir, "cache-dir", "", "snapshot cache directory (default OS user cache / tokenlens)")
	f.BoolVar(&o.Offline, "offline", false, "use ccusage cached pricing for speed (estimates may differ)")
	f.DurationVar(&o.CacheTTL, "cache-ttl", 5*time.Minute, "reuse recent reports for this long; 0 always reloads; r forces refresh")
	f.BoolVar(&o.NoCache, "no-cache", false, "disable memory and on-disk usage snapshots")
	f.StringVar(&o.Theme, "theme", "dark", "dark, light, or ascii")
	f.StringVar(&o.ExportDir, "export-dir", "exports", "directory for filtered CSV/JSON/SVG/PNG exports")
	f.Float64Var(&o.PlanCost, "plan-cost", 0, "configured monthly plan price in startup display currency")
	f.StringVar(&o.PlanAgent, "plan-agent", "", "agent covered by your plan, e.g. claude")
	f.IntVar(&o.BillingDay, "billing-day", 1, "monthly billing start day (1–31, clamped in shorter months)")
	f.StringVar(&o.Currency, "currency", o.Currency, "display currency (e.g. EUR); default TOKENLENS_CURRENCY or USD; fetches ECB reference rate")
	f.BoolVar(&o.Demo, "demo", false, "clearly labeled synthetic demo; no backend needed")
	f.Usage = func() {
		fmt.Fprint(f.Output(), "Tokenlens — a local lens on agent usage\n\nUsage: tokenlens [daily|weekly|monthly] [flags]\n\nNo range flags: current calendar month. Costs originate in USD; --currency converts display amounts.\n")
		f.PrintDefaults()
	}
	if e := f.Parse(args); e != nil {
		return o, e
	}
	if f.NArg() > 0 {
		return o, fmt.Errorf("unexpected argument %q", f.Arg(0))
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
	if o.Theme != "dark" && o.Theme != "light" && o.Theme != "ascii" {
		return o, fmt.Errorf("--theme must be dark, light, or ascii")
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
	loc, e := time.LoadLocation(o.TZ)
	if e != nil {
		return o, fmt.Errorf("invalid timezone %q", o.TZ)
	}
	o.Range, e = datefilter.Resolve(since, until, last, o.Group, now, loc)
	return o, e
}

// Run starts the terminal app and returns a process exit code.
func Run(args []string) int {
	o, e := options(args, time.Now())
	if errors.Is(e, flag.ErrHelp) {
		return 0
	}
	if e != nil {
		fmt.Fprintln(os.Stderr, e)
		return 2
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
