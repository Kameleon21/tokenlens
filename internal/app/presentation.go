package app

import (
	"encoding/json"
	"sort"
	"strings"
	"time"
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
	date := "2 Jan 2006"
	switch m.o.preferences.DateFormat {
	case "us":
		date = "Jan 2, 2006"
	case "iso":
		date = "2006-01-02"
	}
	return t.Format(clock) + " · " + t.Format(date)
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
