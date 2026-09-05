package main

import (
	"encoding/json"
	"fmt"
	"time"
)

func known(v float64) Metric { return Metric{Value: v, Known: true} }

// Deterministic synthetic data; never reads local agent logs.
func demo(r Range, loc *time.Location) Snapshot {
	s := Snapshot{Sections: map[string][]Row{}, Loaded: time.Now()}
	start := time.Date(2026, 9, 1, 0, 0, 0, 0, loc)
	end := start.AddDate(0, 1, -1)
	if r.Since != "" {
		start, _ = parseDate(r.Since, loc)
	}
	if r.Until != "" {
		end, _ = parseDate(r.Until, loc)
	} else {
		end = start.AddDate(0, 1, -1)
	}
	if r.Since == "" {
		start = end.AddDate(0, -1, 1)
	}
	buckets := map[string]map[string]Row{"weekly": {}, "monthly": {}}
	for day, n := start, 0; !day.After(end) && n < 3660; day, n = day.AddDate(0, 0, 1), n+1 {
		row := Row{Name: day.Format("2006-01-02"), Agent: "all"}
		for i, a := range []string{"claude", "codex", "opencode", "gemini"} {
			seed := float64((day.Day()*17+i*31)%90 + 10)
			u := Usage{known(seed * 811), known(seed * 93), known(seed * 2211), known(seed * 41), known(seed * 3156), known(seed * float64(4-i) * .081)}
			model := []string{"claude-sonnet-4-5", "gpt-5", "deepseek-v3", "gemini-2.5-pro"}[i]
			m := Row{Name: model, Usage: u}
			ar := Row{Name: a, Agent: a, Usage: u, Models: []Row{m}}
			row.Agents = append(row.Agents, ar)
			row.Models = append(row.Models, m)
			row.Usage.add(u)
			s.Sections["session"] = append(s.Sections["session"], Row{Name: fmt.Sprintf("%s / studio-%02d", a, n+1), Agent: a, Usage: u, Models: []Row{m}, Metadata: map[string]json.RawMessage{"projectPath": json.RawMessage(fmt.Sprintf("%q", []string{"demo/tokenlens", "demo/website", "demo/api"}[n%3]))}})
		}
		s.Sections["daily"] = append(s.Sections["daily"], row)
		for _, group := range []string{"weekly", "monthly"} {
			key := day.Format("2006-01")
			if group == "weekly" {
				key = day.AddDate(0, 0, -(int(day.Weekday())+6)%7).Format("2006-01-02")
			}
			x := buckets[group][key]
			x.Name = key
			x.Agent = "all"
			x.Usage.add(row.Usage)
			x.Agents = append(x.Agents, row.Agents...)
			x.Models = append(x.Models, row.Models...)
			buckets[group][key] = x
		}
	}
	for group, b := range buckets {
		for _, row := range b {
			row.Models = rank([]Row{row}, "models", "", "")
			row.Agents = mergeAgents(row.Agents)
			s.Sections[group] = append(s.Sections[group], row)
		}
	}
	return s
}

func mergeAgents(rows []Row) []Row {
	by := map[string]Row{}
	for _, r := range rows {
		x := by[r.Agent]
		x.Name = r.Agent
		x.Agent = r.Agent
		x.Usage.add(r.Usage)
		x.Models = append(x.Models, r.Models...)
		by[r.Agent] = x
	}
	out := []Row{}
	for _, r := range by {
		r.Models = rank([]Row{r}, "models", "", "")
		out = append(out, r)
	}
	return out
}
