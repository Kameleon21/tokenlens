package datefilter

import (
	"testing"
	"time"
)

func TestRanges(t *testing.T) {
	loc, _ := time.LoadLocation("America/New_York")
	now := time.Date(2026, 3, 1, 1, 0, 0, 0, time.UTC)
	cases := []struct {
		s, u, g string
		n       int
		want    Range
		bad     bool
	}{
		{"", "", "daily", 0, Range{"2026-02-01", "2026-02-28"}, false},
		{"20240229", "", "daily", 0, Range{"2024-02-29", ""}, false},
		{"", "2024-02-29", "weekly", 0, Range{"", "2024-02-29"}, false},
		{"", "", "daily", 2, Range{"2026-02-27", "2026-02-28"}, false},
		{"", "", "weekly", 1, Range{"2026-02-23", "2026-02-28"}, false},
		{"", "", "monthly", 3, Range{"2025-12-01", "2026-02-28"}, false},
		{"20260230", "", "daily", 0, Range{}, true}, {"2026-2-01", "", "daily", 0, Range{}, true}, {"20260301", "20260201", "daily", 0, Range{}, true}, {"20260301", "", "daily", 1, Range{}, true}, {"", "", "daily", -1, Range{}, true},
	}
	for _, c := range cases {
		r, e := Resolve(c.s, c.u, c.n, c.g, now, loc)
		if (e != nil) != c.bad || !c.bad && r != c.want {
			t.Errorf("%+v: got %+v, %v", c, r, e)
		}
	}
	dst := time.Date(2026, 3, 9, 12, 0, 0, 0, loc)
	r, e := Resolve("", "", 2, "daily", dst, loc)
	if e != nil || r.Since != "2026-03-08" {
		t.Fatalf("DST range: %+v %v", r, e)
	}
}
