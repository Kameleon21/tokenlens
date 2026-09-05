package main

import (
	"fmt"
	"time"
	_ "time/tzdata"
)

type Range struct{ Since, Until string }

func parseDate(s string, loc *time.Location) (time.Time, error) {
	layout := "2006-01-02"
	if len(s) == 8 {
		layout = "20060102"
	} else if len(s) != 10 {
		return time.Time{}, fmt.Errorf("invalid date %q: use YYYY-MM-DD or YYYYMMDD", s)
	}
	t, e := time.ParseInLocation(layout, s, loc)
	if e != nil {
		return t, fmt.Errorf("invalid calendar date %q", s)
	}
	return t, nil
}
func resolveRange(since, until string, last int, group string, now time.Time, loc *time.Location) (Range, error) {
	if last < 0 || last > 10000 {
		return Range{}, fmt.Errorf("--last must be between 1 and 10000")
	}
	if last > 0 && (since != "" || until != "") {
		return Range{}, fmt.Errorf("--last cannot combine with --since or --until")
	}
	now = now.In(loc)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	if last > 0 {
		start := today
		switch group {
		case "daily":
			start = start.AddDate(0, 0, 1-last)
		case "weekly":
			start = start.AddDate(0, 0, -(int(start.Weekday())+6)%7-7*(last-1))
		case "monthly":
			start = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, loc).AddDate(0, 1-last, 0)
		}
		return Range{start.Format("2006-01-02"), today.Format("2006-01-02")}, nil
	}
	if since == "" && until == "" {
		start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, loc)
		return Range{start.Format("2006-01-02"), start.AddDate(0, 1, -1).Format("2006-01-02")}, nil
	}
	r := Range{}
	for i, s := range []string{since, until} {
		if s == "" {
			continue
		}
		t, e := parseDate(s, loc)
		if e != nil {
			return r, e
		}
		if i == 0 {
			r.Since = t.Format("2006-01-02")
		} else {
			r.Until = t.Format("2006-01-02")
		}
	}
	if r.Since != "" && r.Until != "" && r.Since > r.Until {
		return r, fmt.Errorf("--since must not be after --until")
	}
	return r, nil
}
func (r Range) String() string {
	s, u := r.Since, r.Until
	if s == "" {
		s = "beginning"
	}
	if u == "" {
		u = "latest"
	}
	return s + " → " + u
}
