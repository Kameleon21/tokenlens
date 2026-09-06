package app

import (
	"encoding/json"
	"sort"
	"strings"
	"time"

	"github.com/Kameleon21/tokenlens/internal/datefilter"
)

var modelSorts = []string{"cost_desc", "cost_asc", "name_asc", "name_desc"}
var sessionSorts = []string{"cost_desc", "cost_asc", "name_asc", "name_desc", "newest", "oldest"}

// Metadata timestamps are parsed at ingestion, including for older disk caches.
// Names and date-only periods are never interpreted as session activity times.
func (r *Row) prepareTimes() {
	r.metadataTimes = nil
	r.firstActivity, r.lastActivity = time.Time{}, time.Time{}
	for key, raw := range r.Metadata {
		var value string
		if json.Unmarshal(raw, &value) != nil {
			continue
		}
		if t, err := time.Parse(time.RFC3339Nano, value); err == nil {
			if r.metadataTimes == nil {
				r.metadataTimes = make(map[string]time.Time)
			}
			r.metadataTimes[key] = t
		}
	}
	r.firstActivity = r.metadataTimes["firstActivity"]
	r.lastActivity = r.metadataTimes["lastActivity"]
}
func prepareRowTimes(rows []Row) {
	for i := range rows {
		rows[i].prepareTimes()
		prepareRowTimes(rows[i].Agents)
		prepareRowTimes(rows[i].Models)
	}
}
func (s *Snapshot) prepareTimes() {
	for _, rows := range s.Sections {
		prepareRowTimes(rows)
	}
}
func (m model) formatTimestamp(t time.Time) string {
	if t.IsZero() {
		return "unavailable"
	}
	loc := m.displayLocation
	if loc == nil {
		loc = time.UTC
	}
	t = t.In(loc)
	clock := "15:04"
	if m.o.preferences.ClockFormat == "12h" {
		clock = "3:04 PM"
	}
	return t.Format(clock) + " · " + t.Format(m.dateLayout())
}
func (m model) dateLayout() string {
	switch m.o.preferences.DateFormat {
	case "us":
		return "Jan 2, 2006"
	case "iso":
		return "2006-01-02"
	default:
		return "2 Jan 2006"
	}
}

// Calendar periods have no timezone: converting midnight would shift their day.
func (m model) formatPeriod(value string) string {
	if t, err := time.Parse("2006-01-02", value); err == nil {
		return t.Format(m.dateLayout())
	}
	if t, err := time.Parse("2006-01", value); err == nil && m.o.preferences.DateFormat != "iso" {
		return t.Format("Jan 2006")
	}
	return value
}
func (m model) formatRange(r datefilter.Range) string {
	since, until := "beginning", "latest"
	if r.Since != "" {
		since = m.formatPeriod(r.Since)
	}
	if r.Until != "" {
		until = m.formatPeriod(r.Until)
	}
	return since + " → " + until
}
func (m model) rowLabel(r Row) string {
	if m.view == 0 {
		return m.formatPeriod(r.Name)
	}
	return r.Name
}
func (m model) exchangeLabel() string {
	x := m.fx
	x.Date = m.formatPeriod(x.Date)
	return x.label()
}
func (m model) rangeInput() string {
	since, until := "*", "*"
	if m.o.Range.Since != "" {
		since = m.formatPeriod(m.o.Range.Since)
	}
	if m.o.Range.Until != "" {
		until = m.formatPeriod(m.o.Range.Until)
	}
	return since + " → " + until
}
func (m model) canonicalDate(value string) string {
	value = strings.TrimSpace(value)
	if t, err := time.Parse(m.dateLayout(), value); err == nil {
		return t.Format("2006-01-02")
	}
	return value
}
func (m model) rangeHelp() string {
	return "Two inclusive dates: " + m.formatPeriod("2026-09-01") + " → " + m.formatPeriod("2026-09-30") +
		"\nSeparate dates with → or to; * for an open bound.\nISO dates, month, or last N also accepted.\nN uses the current daily / weekly / monthly grouping."
}

func (m model) metadataValue(r Row, key string) string {
	if t, ok := r.metadataTimes[key]; ok {
		return m.formatTimestamp(t)
	}
	return string(r.Metadata[key])
}
func (m model) sessionTimes(r Row) string {
	if m.view != 4 {
		return ""
	}
	return "Started " + m.formatTimestamp(r.firstActivity) + "\nLast activity " + m.formatTimestamp(r.lastActivity) + "\n"
}
func (m model) tabSort() string {
	value := m.o.preferences.ModelsSort
	if m.view == 4 {
		value = m.o.preferences.SessionsSort
	}
	if value == "" {
		return "cost_desc"
	}
	return value
}
func (m model) sortLabel() string {
	if m.view != 2 && m.view != 4 {
		return []string{"largest first", "smallest first", "name A–Z"}[m.sortMode]
	}
	return map[string]string{
		"cost_desc": "Cost ↓ most expensive", "cost_asc": "Cost ↑ cheapest",
		"name_asc": "Name A–Z", "name_desc": "Name Z–A",
		"newest": "Started ↓ newest", "oldest": "Started ↑ oldest",
	}[m.tabSort()]
}
func sortRows(rows []Row, order string) {
	sort.SliceStable(rows, func(i, j int) bool {
		a, b := rows[i], rows[j]
		switch order {
		case "cost_asc", "cost_desc":
			x, y := a.Usage.Cost, b.Usage.Cost
			if x.Known != y.Known {
				return x.Known
			}
			if x.Known && x.Value != y.Value {
				if order == "cost_asc" {
					return x.Value < y.Value
				}
				return x.Value > y.Value
			}
		case "newest", "oldest":
			x, y := a.firstActivity, b.firstActivity
			if x.IsZero() != y.IsZero() {
				return !x.IsZero()
			}
			if !x.Equal(y) {
				if order == "oldest" {
					return x.Before(y)
				}
				return x.After(y)
			}
		case "name_asc", "name_desc":
			x, y := strings.ToLower(a.Name), strings.ToLower(b.Name)
			if x != y {
				if order == "name_desc" {
					return x > y
				}
				return x < y
			}
		}
		// Stable identity tie-breakers, independent of display format and metric mode.
		if a.Name != b.Name {
			return a.Name < b.Name
		}
		if a.Agent != b.Agent {
			return a.Agent < b.Agent
		}
		if !a.firstActivity.Equal(b.firstActivity) {
			return a.firstActivity.Before(b.firstActivity)
		}
		if !a.lastActivity.Equal(b.lastActivity) {
			return a.lastActivity.Before(b.lastActivity)
		}
		return false // Exact duplicate identities retain source order.
	})
}
func (m model) displayHelp() string {
	return "s  Sort: cost ↓/↑, name A–Z/Z–A\n   Sessions also: started newest/oldest\nD  Date: European / U.S. / ISO (" + m.o.preferences.DateFormat + ")\nH  Clock: 12h / 24h (" + m.o.preferences.ClockFormat + ")\n   Tab sorting saved; missing costs/times last."
}

// Machine exports retain full precision and an explicit UTC offset.
func machineTimestamp(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339Nano)
}
