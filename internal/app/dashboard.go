package app

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

var borderColor = lipgloss.Color("#394555")
var surface = lipgloss.Color("#19212C")

// fit bounds every pane by terminal cells, including ANSI and wide Unicode.
func fit(s string, w, h int) string {
	w = max(1, w)
	h = max(1, h)
	lines := strings.Split(s, "\n")
	out := make([]string, h)
	for i := 0; i < h; i++ {
		if i < len(lines) {
			out[i] = clip(lines[i], w)
		}
		out[i] += strings.Repeat(" ", max(0, w-ansi.StringWidth(out[i])))
	}
	return strings.Join(out, "\n")
}
func pane(title, caption, body string, w, h int, focused bool) string {
	inner := max(1, w-6)
	header := bright.Render(title)
	if caption != "" && ansi.StringWidth(header)+len(caption)+3 < inner {
		header += strings.Repeat(" ", inner-ansi.StringWidth(header)-len(caption)) + muted.Render(caption)
	}
	c := borderColor
	if focused {
		c = lipgloss.Color(paletteFor(activeTheme).accent)
	}
	return lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(c).Padding(0, 2).Render(fit(header+"\n\n"+body, inner, h-2))
}
func chip(s string, selected bool, c lipgloss.Color) string {
	style := lipgloss.NewStyle().Padding(0, 1).Foreground(c)
	if selected {
		style = style.Background(surface).Bold(true)
		s = "● " + s
	} else {
		s = "○ " + s
	}
	return style.Render(s)
}
func (m model) dashboardView() string {
	if m.width < 96 || m.height < 32 {
		return m.compactView()
	}
	w, h := m.width-4, m.height-2
	var head strings.Builder
	mode := "LOCAL SNAPSHOT"
	if m.o.Demo {
		mode = "DEMO · SYNTHETIC"
	}
	left := bright.Render("◈  T O K E N L E N S") + "   " + muted.Render(mode)
	right := m.themeIndicator()
	left = clip(left, max(1, w-ansi.StringWidth(right)-2))
	head.WriteString(left + strings.Repeat(" ", max(2, w-ansi.StringWidth(left)-ansi.StringWidth(right))) + right + "\n")
	fresh := "Waiting for first snapshot"
	if !m.s.Loaded.IsZero() {
		fresh = "Snapshot " + m.formatTimestamp(m.s.Loaded)
		if m.cached {
			fresh = "Cached " + fresh
		}
	}
	if m.loading {
		fresh = m.spin.View() + fmt.Sprintf(" Refreshing usage · %.0fs", time.Since(m.loadingSince).Seconds())
		if m.cached {
			fresh += " · cached " + m.formatTimestamp(m.s.Loaded)
		}
	}
	head.WriteString(muted.Render(m.formatRange(m.o.Range)+"  ·  "+m.o.TZ) + strings.Repeat(" ", max(2, w-len(m.formatRange(m.o.Range)+"  ·  "+m.o.TZ)-ansi.StringWidth(fresh))) + muted.Render(fresh) + "\n")
	head.WriteString(muted.Render(strings.Repeat("─", w)) + "\n")
	filters := chip("All agents", m.agent == "", lipgloss.Color(paletteFor(activeTheme).accent))
	agentNames := names(m.s, "agent")
	for _, a := range agentNames[1:] {
		next := chip(safe(a), m.agent == a, color(a))
		if ansi.StringWidth(filters)+ansi.StringWidth(next) > w-19 {
			filters += muted.Render("  … [a] cycle")
			break
		}
		filters += next
	}
	head.WriteString(muted.Render("AGENT FILTER  ") + filters + "\n")
	modelName := m.modelFilter
	if modelName == "" {
		modelName = "All models"
	}
	head.WriteString(muted.Render("[a] cycle agents   [f] model: ") + accent.Render(safe(modelName)) + muted.Render("   [x] clear filters") + "\n\n")
	u := total(filtered(m.s.Sections["daily"], m.agent, m.modelFilter))
	gap := 2
	cardW := (w - 2*gap) / 3
	costBody := accent.Bold(true).Render(m.fx.format(u.Cost)) + "\n" + bright.Render(format(u.Tokens, false)) + muted.Render(" total tokens") + "\n" + muted.Render(clip(m.pricingStatus(), cardW-6))

	if m.showPlan {
		costBody = m.planSummary()
	}
	costAgents := rank(m.s.Sections["daily"], "agents", m.agent, m.modelFilter)
	sort.Slice(costAgents, func(i, j int) bool { return costAgents[i].Usage.Cost.Value > costAgents[j].Usage.Cost.Value })
	top := muted.Render("Unavailable") + "\n\n" + muted.Render("No agent breakdown in this snapshot")
	if len(costAgents) > 0 {
		a := costAgents[0]
		top = lipgloss.NewStyle().Foreground(color(a.Name)).Bold(true).Render(safe(a.Name)) + "\n" + m.fx.format(a.Usage.Cost) + muted.Render("  ·  "+shareOf(a.Usage.Cost, u.Cost)+" of estimated cost") + "\n" + muted.Render("Largest known estimated cost contributor")
	}
	cacheShare := "unavailable"
	den := u.Input.Value + u.Read.Value + u.Write.Value
	if u.Read.Known && u.Input.Known && u.Write.Known && !u.Read.Partial && !u.Input.Partial && !u.Write.Partial && den > 0 {
		cacheShare = fmt.Sprintf("%.1f%%", u.Read.Value/den*100)
	}
	cacheBody := bright.Render(cacheShare) + muted.Render(" of input-side tokens") + "\n" + format(u.Read, false) + muted.Render(" cache-read tokens") + "\n" + muted.Render("Monetary savings unavailable")
	head.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, pane("ESTIMATED COST · "+m.fx.Currency, "", costBody, cardW, 7, false), "  ", pane("TOP CONSUMING AGENT", "", top, cardW, 7, false), "  ", pane("CACHE READ SHARE", "", cacheBody, w-2*cardW-4, 7, false)) + "\n")
	tabs := []string{}
	for i, title := range views {
		tabs = append(tabs, chip(fmt.Sprintf("%d %s", i+1, title), i == m.view, lipgloss.Color(paletteFor(activeTheme).accent)))
	}
	head.WriteString(strings.Join(tabs, " ") + "\n")
	fx := muted.Render("Native USD estimates · [c] cost / tokens · [s] sort")
	if m.o.Currency != "USD" {
		fx = muted.Render(m.exchangeStatus())
	}
	if m.o.Offline {
		fx += muted.Render(" · OFFLINE: cached ccusage pricing")
	}
	if m.notice != "" {
		fx += muted.Render("  ·  " + m.notice)
	}
	head.WriteString(clip(fx, w) + "\n")
	prefix := head.String()
	bodyH := max(5, h-lipgloss.Height(prefix)-1)
	var body string
	switch {
	case m.help:
		body = pane("CONTROLS", "esc close", m.helpContent(bodyH-4), w, bodyH, true)
	case m.info != "":
		body = pane("DATA AVAILABILITY", "esc close", m.info, w, bodyH, true)
	case m.exporting:
		body = pane("EXPORT FILTERED VIEW", "esc cancel", "[1] JSON · complete metrics and currency metadata\n[2] CSV · spreadsheet-ready values\n[3] SVG · vector chart\n[4] PNG · chart image\n\nExports use the selected range, filters, grouping and currency.\nDirectory: "+safe(m.o.ExportDir), w, bodyH, true)
	case m.editing != "":
		body = pane("DATE RANGE", "enter apply · esc cancel", m.input.View()+"\n\n"+muted.Render(m.rangeHelp())+"\n\n"+safe(m.err), w, bodyH, true)
	case m.err != "":
		msg := safe(m.err)
		if strings.Contains(m.err, "not installed") {
			msg = "Tokenlens needs ccusage to read your local usage.\n\nInstall Bun for automatic ccusage launch, or install ccusage:\n  npm install -g ccusage@20.0.20\n\nThen press r here to retry, or restart with:\n  go run . --currency " + m.o.Currency + "\n\nTo explore with sample data only:\n  go run . --demo --currency " + m.o.Currency
		}
		body = pane("CONNECT YOUR USAGE", "r retry · esc dismiss", msg, w, bodyH, true)
	case m.view != 0 || m.activityDetail || m.details:
		body = m.drilldown(w, bodyH)
	default:
		cw := (w - 2) / 2
		topH := (bodyH - 1) / 2
		bottomH := bodyH - topH - 1
		titles := []string{"Spend & model split", "Model comparison", "Sessions & repository cost", "Activity by agent"}
		contents := []string{m.stackedChart(cw-6, topH-4), m.modelChart(cw-6, topH-4), m.sessionChart(cw-6, bottomH-4), m.activityChart(w-cw-8, bottomH-4)}
		order := []int{0, 1, 2, 3}
		if m.layout == 1 {
			order = []int{3, 0, 1, 2}
		}
		captions := []string{"← → inspect · enter open", "3 · models", "5 · sessions", "d / w / m · periods"}
		row := func(a, b, height int) string {
			return lipgloss.JoinHorizontal(lipgloss.Top, pane(titles[a], captions[a], contents[a], cw, height, m.widget == a), "  ", pane(titles[b], captions[b], contents[b], w-cw-2, height, m.widget == b))
		}
		body = row(order[0], order[1], topH) + "\n" + row(order[2], order[3], bottomH)
	}
	footer := muted.Render("Ctrl+T themes  [ / ] widgets  enter open  v layout  e currency  p preset  b plan  o export  c metric  ? help  q quit")
	if m.help {
		footer = muted.Render("↑ ↓ scroll · home/end · esc close · ? close · q quit")
	}
	content := fit(prefix+body, w, h-1) + "\n" + fit(footer, w, 1)
	return themeRender(lipgloss.NewStyle().Foreground(ink).Padding(1, 2).Render(content), m.o.Theme, m.width, m.height)
}
func shareOf(v, total Metric) string {
	if !v.Known || v.Partial || !total.Known || total.Partial || total.Value <= 0 {
		return "—"
	}
	return fmt.Sprintf("%.1f%%", 100*v.Value/total.Value)
}
func (m model) ranked(kind string) []Row {
	var rows []Row
	if kind == "agents" {
		rows = rank(m.s.Sections["daily"], kind, m.agent, m.modelFilter)
	} else {
		rows = rank(filtered(m.s.Sections["daily"], m.agent, ""), kind, "", m.modelFilter)
	}
	sort.Slice(rows, func(i, j int) bool {
		if m.sortMode == 2 {
			return rows[i].Name < rows[j].Name
		}
		a, b := m.value(rows[i]).Value, m.value(rows[j]).Value
		if a == b {
			return rows[i].Name < rows[j].Name
		}
		if m.sortMode == 1 {
			return a < b
		}
		return a > b
	})
	return rows
}
func (m model) bars(rows []Row, w, h int, selected bool) string {
	if len(rows) == 0 {
		return muted.Render("No data for the selected range and filters.\n[t] change range   [x] clear filters")
	}
	sum := Metric{}
	peak := 0.0
	for _, r := range rows {
		v := m.value(r)
		sum.add(v)
		peak = max(peak, v.Value)
	}
	lines := []string{}
	stride := 2
	if h >= len(rows)*3 {
		stride = 3
	}
	limit := max(1, h/stride)
	start := 0
	if selected {
		start = max(0, min(m.cursor, len(rows)-1)-limit+1)
	}
	end := min(len(rows), start+limit)
	for i := start; i < end; i++ {
		r := rows[i]
		v := m.value(r)
		val := m.formatMetric(v, m.cost && m.view != 3) + "  " + shareOf(v, sum)
		label := safe(r.Name)
		if selected && m.view == 0 {
			label = safe(m.formatPeriod(r.Name))
		}
		prefix := ""
		if selected {
			prefix = "  "
			if i == m.cursor {
				prefix = "› "
			}
		}
		lw := max(4, w-ansi.StringWidth(val)-2)
		left := clip(prefix+label, lw)
		line := left + strings.Repeat(" ", max(1, w-ansi.StringWidth(left)-ansi.StringWidth(val))) + val
		fill := 0
		if peak > 0 && v.Known {
			fill = int(v.Value / peak * float64(w))
		}
		c := color(r.Agent)
		if r.Agent == "all" {
			c = lipgloss.Color(paletteFor(activeTheme).accent)
		} else if r.Agent == "" {
			c = color(r.Name)
		}
		lines = append(lines, line, lipgloss.NewStyle().Foreground(c).Render(strings.Repeat("━", fill))+muted.Render(strings.Repeat("─", max(0, w-fill))))
		if stride == 3 {
			lines = append(lines, "")
		}
	}
	if end < len(rows) && len(lines) < h {
		lines = append(lines, muted.Render(fmt.Sprintf("+ %d more · open to explore", len(rows)-end)))
	}
	return strings.Join(lines, "\n")
}
func (m model) shareChart(rows []Row, w, h int) string {
	if len(rows) == 0 {
		return m.bars(rows, w, h, false)
	}
	sum := Metric{}
	for _, r := range rows {
		sum.add(m.value(r))
	}
	stacked := ""
	if sum.Known && !sum.Partial && sum.Value > 0 {
		used := 0
		for i, r := range rows {
			n := int(math.Round(m.value(r).Value / sum.Value * float64(w)))
			n = min(n, w-used)
			if i == len(rows)-1 {
				n = w - used
			}
			stacked += lipgloss.NewStyle().Foreground(color(r.Name)).Render(strings.Repeat("█", max(0, n)))
			used += n
		}
	} else {
		stacked = muted.Render("Shares unavailable for incomplete metrics")
	}
	return stacked + "\n\n" + m.bars(rows, w, max(1, h-2), false)
}
func (m model) modelChart(w, h int) string { return m.ringChart(m.ranked("models"), w, h) }
func (m model) ringChart(rows []Row, w, h int) string {
	if w < 50 || h < 10 || len(rows) == 0 {
		return m.bars(rows, w, h, false)
	}
	sum := Metric{}
	for _, r := range rows {
		sum.add(m.value(r))
	}
	if !sum.Known || sum.Partial || sum.Value <= 0 {
		return m.bars(rows, w, h, false)
	}
	ring := []string{}
	for y := -4; y <= 4; y++ {
		line := ""
		for x := -9; x <= 9; x++ {
			dx, dy := float64(x)/2, float64(y)
			r := math.Hypot(dx, dy)
			if r < 2.4 || r > 4.6 {
				line += " "
				continue
			}
			angle := (math.Atan2(dy, dx) + math.Pi) / (2 * math.Pi)
			cum := 0.0
			idx := len(rows) - 1
			for i, row := range rows {
				cum += m.value(row).Value / sum.Value
				if angle <= cum {
					idx = i
					break
				}
			}
			line += lipgloss.NewStyle().Foreground(color(rows[idx].Name)).Render("█")
		}
		ring = append(ring, line)
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, strings.Join(ring, "\n"), "   ", m.bars(rows, w-22, h, false))
}
func (m model) sessionChart(w, h int) string {
	rows := filtered(m.s.Sections["session"], m.agent, m.modelFilter)
	by := map[string]Row{}
	attributed := 0
	for _, r := range rows {
		project := ""
		for _, key := range []string{"projectPath", "cwd", "project", "directory"} {
			if v, ok := r.Metadata[key]; ok {
				_ = json.Unmarshal(v, &project)
				if project != "" {
					break
				}
			}
		}
		if project != "" {
			attributed++
			x := by[project]
			x.Name = project
			x.Usage.add(r.Usage)
			by[project] = x
		}
	}
	label := "Repository attribution unavailable · showing sessions"
	if attributed == len(rows) && len(rows) > 0 {
		rows = nil
		for _, r := range by {
			rows = append(rows, r)
		}
		label = "Grouped by reported repository / working directory"
	}
	sort.Slice(rows, func(i, j int) bool {
		a, b := m.value(rows[i]).Value, m.value(rows[j]).Value
		if a == b {
			return rows[i].Name < rows[j].Name
		}
		return a > b
	})
	chart := m.bars(rows, w, max(1, h-2), false)
	if attributed > 0 && attributed == len(filtered(m.s.Sections["session"], m.agent, m.modelFilter)) {
		chart = m.ringChart(rows, w, max(1, h-2))
	}
	return muted.Render(clip(label, w)) + "\n\n" + chart
}
func (m model) activityChart(w, h int) string {
	daily := append([]Row(nil), m.s.Sections["daily"]...)
	sort.Slice(daily, func(i, j int) bool { return daily[i].Name < daily[j].Name })
	if len(daily) == 0 {
		return muted.Render("No activity in this snapshot.\n[t] change range or [r] refresh")
	}
	maxDays := max(1, (w-30)/2)
	if len(daily) > maxDays {
		daily = daily[len(daily)-maxDays:]
	}
	agents := names(m.s, "agent")[1:]
	if m.agent != "" {
		agents = []string{m.agent}
	}
	if len(agents) == 0 {
		return muted.Render("Per-agent activity unavailable")
	}
	peak := 0.0
	values := map[string][]Metric{}
	for _, a := range agents {
		for _, d := range daily {
			v := Metric{Known: true}
			rs := filtered([]Row{d}, a, m.modelFilter)
			if len(rs) > 0 {
				v = m.value(rs[0])
			}
			peak = max(peak, v.Value)
			values[a] = append(values[a], v)
		}
	}
	labelWidth := 14
	axis := strings.Repeat(" ", labelWidth)
	for i, d := range daily {
		if i%5 == 0 && len(d.Name) >= 10 {
			axis += d.Name[8:10]
		} else {
			axis += "  "
		}
	}
	lines := []string{muted.Render(axis)}
	for _, a := range agents[:min(len(agents), max(1, h-4))] {
		label := clip(safe(a), labelWidth-1)
		line := label + strings.Repeat(" ", labelWidth-ansi.StringWidth(label))
		sum := Metric{}
		for _, v := range values[a] {
			sum.add(v)
			cell := "· "
			if !v.Known {
				cell = "? "
			} else if v.Value > 0 && peak > 0 {
				shade := []string{"░ ", "▒ ", "▓ ", "█ "}
				cell = shade[min(3, int(v.Value/peak*3))]
			}
			line += lipgloss.NewStyle().Foreground(color(a)).Render(cell)
		}
		line += "  " + m.formatMetric(sum, m.cost)
		lines = append(lines, clip(line, w))
	}
	lines = append(lines, "", muted.Render(clip(m.formatPeriod(daily[0].Name)+" → "+m.formatPeriod(daily[len(daily)-1].Name)+" · active days · low ░▒▓█ high", w)))
	return strings.Join(lines, "\n")
}
func (m model) drilldown(w, h int) string {
	title := views[m.view]
	if m.view == 0 {
		title = "Activity · " + m.o.Group
	}
	rows := m.rows()
	leftW := (w * 3) / 5
	rightW := w - leftW - 2
	details := muted.Render("Select a row to inspect its metrics.")
	if len(rows) > 0 {
		r := rows[min(m.cursor, len(rows)-1)]
		details = bright.Render(safe(m.rowLabel(r))) + "\n" + m.sessionTimes(r) + "\n" + muted.Render("ESTIMATED COST · "+m.fx.Currency) + "\n" + accent.Bold(true).Render(m.fx.format(r.Usage.Cost)) + "\n\n" + muted.Render("TOKEN BREAKDOWN") + "\n"
		for _, v := range []struct {
			name string
			v    Metric
		}{{"Total", r.Usage.Tokens}, {"Input", r.Usage.Input}, {"Output", r.Usage.Output}, {"Cache read", r.Usage.Read}, {"Cache write", r.Usage.Write}} {
			details += fmt.Sprintf("%-16s %s\n", v.name, format(v.v, false))
		}
		if len(r.Models) > 0 {
			details += "\n" + muted.Render("MODELS") + "\n"
			for _, mr := range r.Models {
				details += safe(mr.Name) + "\n  " + m.formatMetric(m.value(mr), m.cost) + "\n"
			}
		}
		if len(r.Metadata) > 0 {
			details += "\n" + muted.Render("SESSION METADATA") + "\n"
			keys := []string{}
			for k := range r.Metadata {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				details += safe(k+": "+m.metadataValue(r, k)) + "\n"
			}
		}
	}
	if m.view == 3 {
		details = m.cacheChart(rightW-6, h-4)
	}
	caption := "[s] " + m.sortLabel()
	return lipgloss.JoinHorizontal(lipgloss.Top, pane(title, "", muted.Render(caption)+"\n"+m.bars(rows, leftW-6, h-5, true), leftW, h, true), "  ", pane("Inspector", "↑ ↓ select · esc back", details, rightW, h, false))
}
func (m model) helpText() string {
	return m.displayHelp() + "\n\n1–5 / tab     Overview, agents, models, tokens/cache, sessions\nd / w / m     Daily, weekly, monthly grouping\na / f         Cycle agent / model filter independently\nx             Clear both filters\nc / s         Cost or tokens / sorting order\n[ / ]         Focus overview widget\nenter         Open focused widget or inspect a row\nv             Switch overview layout\ne / ctrl+t    Currency / searchable theme picker\nn             Compact k/M/B token labels (inspector stays exact)\np / b         Date preset / configured plan comparison\no             Export filtered JSON, CSV, SVG, PNG\n← → / hover   Inspect daily stacked bars\nh             Explain unavailable hourly / 5-hour data\n↑ ↓ / j k     Select rows; home/end jump\nt             Edit date range\nr             Refresh usage and exchange rate\nq / ctrl+c    Quit\n\n" + m.exchangeStatus() + "\n\nCost is estimated. Cache savings and spend changes require additional data."
}

func (m model) helpContent(height int) string {
	lines := strings.Split(m.helpText(), "\n")
	height = max(1, height)
	start := max(0, min(m.helpOffset, max(0, len(lines)-height)))
	return strings.Join(lines[start:min(len(lines), start+height)], "\n")
}
