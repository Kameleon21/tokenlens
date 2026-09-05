package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	tea "github.com/charmbracelet/bubbletea"
	"os"
	"time"
)

type Options struct {
	Group, TZ, Bin string
	Range          Range
	Demo           bool
}

func options(args []string, now time.Time) (Options, error) {
	o := Options{Group: "daily", TZ: "UTC", Bin: "ccusage"}
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
	f.BoolVar(&o.Demo, "demo", false, "clearly labeled synthetic demo; no backend needed")
	f.Usage = func() {
		fmt.Fprint(f.Output(), "Tokenlens — a local lens on agent usage\n\nUsage: tokenlens [daily|weekly|monthly] [flags]\n\nNo range flags: current calendar month. Costs are estimates in USD.\n")
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
	loc, e := time.LoadLocation(o.TZ)
	if e != nil {
		return o, fmt.Errorf("invalid timezone %q", o.TZ)
	}
	o.Range, e = resolveRange(since, until, last, o.Group, now, loc)
	return o, e
}
func main() {
	o, e := options(os.Args[1:], time.Now())
	if errors.Is(e, flag.ErrHelp) {
		return
	}
	if e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(2)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if _, e = tea.NewProgram(newModel(ctx, o), tea.WithAltScreen()).Run(); e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
}
