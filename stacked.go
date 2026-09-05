package main

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func (m model) chartPeriods() []Row {
	rows := filtered(m.s.Sections[m.o.Group], m.agent, m.modelFilter)
	sort.Slice(rows, func(i, j int) bool { return rows[i].Name < rows[j].Name })
	return rows
}
func (m model) stackedChart(w, h int) string {
	rows := m.chartPeriods()
	if len(rows) == 0 {
		return muted.Render("No activity for the selected range.\n[t] range · [p] preset")
	}
	count := max(1, w/3)
	cursor := min(m.dayCursor, len(rows)-1)
	start := max(0, cursor-count+1)
	end := min(len(rows), start+count)
	rows = rows[start:end]
	peak := 0.0
	totals := make([]float64, len(rows))
	complete := true
	for i, r := range rows {
		if len(r.Models) == 0 {
			complete = false
		}
		for _, mr := range r.Models {
			v := m.value(mr)
			if !v.Known || v.Partial {
				complete = false
			}
			totals[i] += v.Value
		}
		peak = max(peak, totals[i])
	}
	if !complete {
		return muted.Render("Model-level metrics incomplete; stacked shares unavailable.\nOpen period details to inspect reported totals.")
	}
	chartH := max(1, h-5)
	lines := []string{muted.Render("Peak " + m.formatMetric(known(peak), m.cost) + " · " + m.o.Group + " · ← → select")}
	for y := chartH - 1; y >= 0; y-- {
		line := ""
		for i, r := range rows {
			threshold := (float64(y) + 0.5) / float64(chartH) * peak
			cell := "  "
			if peak > 0 && threshold <= totals[i] {
				sum := 0.0
				for _, mr := range r.Models {
					sum += m.value(mr).Value
					if sum >= threshold {
						cell = lipgloss.NewStyle().Foreground(color(mr.Name)).Render("██")
						break
					}
				}
			}
			line += cell + " "
		}
		lines = append(lines, line)
	}
	axis := ""
	for i, r := range rows {
		label := r.Name
		if len(label) > 2 {
			label = label[len(label)-2:]
		}
		if start+i == cursor {
			axis += accent.Bold(true).Render(label) + " "
		} else {
			axis += muted.Render(label) + " "
		}
	}
	lines = append(lines, axis)
	selected := rows[cursor-start]
	parts := []string{}
	for _, mr := range selected.Models {
		parts = append(parts, lipgloss.NewStyle().Foreground(color(mr.Name)).Render(safe(mr.Name))+" "+m.formatMetric(m.value(mr), m.cost))
	}
	lines = append(lines, bright.Render(selected.Name)+"  "+m.formatMetric(known(totals[cursor-start]), m.cost), clip(strings.Join(parts, " · "), w))
	return strings.Join(lines, "\n")
}
func (m model) cacheChart(w, h int) string {
	u := total(filtered(m.s.Sections["daily"], m.agent, m.modelFilter))
	vals := []Row{{Name: "Uncached input", Usage: Usage{Tokens: u.Input}}, {Name: "Cache read", Usage: Usage{Tokens: u.Read}}, {Name: "Cache write", Usage: Usage{Tokens: u.Write}}}
	sum := u.Input.Value + u.Read.Value + u.Write.Value
	if !u.Input.Known || !u.Read.Known || !u.Write.Known || u.Input.Partial || u.Read.Partial || u.Write.Partial || sum == 0 {
		return muted.Render("Cache breakdown unavailable or incomplete.")
	}
	ratio := u.Read.Value / sum
	filled := int(math.Round(float64(w) * ratio))
	bar := accent.Render(strings.Repeat("█", filled)) + muted.Render(strings.Repeat("░", w-filled))
	n := m
	n.cost = false
	n.view = 3
	return bright.Render(fmt.Sprintf("%.1f%% cache reads", ratio*100)) + muted.Render(" / input-side tokens") + "\n" + bar + "\n\n" + n.ringChart(vals, w, max(1, h-6)) + "\n" + muted.Render("Savings unavailable: per-model uncached pricing is required.")
}
