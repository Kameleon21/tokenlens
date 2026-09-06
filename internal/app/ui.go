package app

import (
	"context"
	"fmt"
	"github.com/Kameleon21/tokenlens/internal/datefilter"
	"hash/fnv"
	"net/http"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

var ink = lipgloss.Color("#E3E9F3")
var muted = lipgloss.NewStyle().Foreground(lipgloss.Color("#8794A9"))
var accent = lipgloss.NewStyle().Foreground(lipgloss.Color(paletteFor(activeTheme).accent))
var bright = lipgloss.NewStyle().Foreground(ink).Bold(true)
var views = []string{"Overview", "Agents", "Models", "Tokens / cache", "Sessions"}

type loadedMsg struct {
	s   Snapshot
	err error
	id  int
	r   datefilter.Range
}
type exchangeMsg struct {
	exchange Exchange
	err      error
	id       int
}
type model struct {
	helpOffset                            int
	displayLocation                       *time.Location
	prices                                priceCatalog
	priceLoading                          bool
	priceAttempt                          time.Time
	priceErr                              string
	choosingTheme                         bool
	themeOriginal                         string
	themeCursor                           int
	themeQuery                            textinput.Model
	reports                               map[datefilter.Range]Snapshot
	fxRequest                             int
	fxTarget                              string
	exchanges                             map[string]Exchange
	compactNumbers                        bool
	info                                  string
	cached                                bool
	loadingSince                          time.Time
	dayCursor, preset                     int
	showPlan, exporting                   bool
	notice                                string
	widget, layout                        int
	activityDetail                        bool
	fx                                    Exchange
	fxLoading                             bool
	fxErr                                 string
	fxCancel                              context.CancelFunc
	ctx                                   context.Context
	o                                     Options
	s                                     Snapshot
	width, height, view, cursor, sortMode int
	cost, loading, details, help          bool
	err                                   string
	spin                                  spinner.Model
	request                               int
	cancel                                context.CancelFunc
	pending                               datefilter.Range
	agent, modelFilter, editing           string
	input                                 textinput.Model
}

func newModel(ctx context.Context, o Options) model {
	if o.Theme == "" {
		o.Theme = "dark"
	}
	applyTheme(o.Theme)
	if o.Currency == "" {
		o.Currency = "USD"
	}
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = accent
	ti := textinput.New()
	ti.CharLimit = 200
	ti.Width = 55
	layout := 0
	if o.preferences.Layout == "stacked" {
		layout = 1
	}
	var prices priceCatalog
	if o.managedPrices {
		prices = initialPrices(o)
		o.priceRevision = prices.revision()
	}
	loc, err := time.LoadLocation(o.TZ)
	if err != nil {
		loc = time.UTC
	}
	defaults := defaultPreferences()
	if o.preferences.DateFormat == "" {
		o.preferences.DateFormat = defaults.DateFormat
	}
	if o.preferences.ClockFormat == "" {
		o.preferences.ClockFormat = defaults.ClockFormat
	}
	if o.preferences.SessionsSort == "" {
		o.preferences.SessionsSort = defaults.SessionsSort
	}
	if o.preferences.ModelsSort == "" {
		o.preferences.ModelsSort = defaults.ModelsSort
	}
	return model{displayLocation: loc, prices: prices, ctx: ctx, o: o, width: 100, height: 32, cost: o.preferences.Display != "tokens", compactNumbers: o.preferences.CompactNumbers, layout: layout, spin: sp, input: ti, fx: initialExchange(o, time.Now())}
}
func (m model) Init() tea.Cmd {
	initial := func() tea.Msg { return "initial-load" }
	if m.o.managedPrices && !m.o.Offline {
		return tea.Batch(initial, priceTick())
	}
	return initial
}
func (m *model) refresh(r datefilter.Range, force ...bool) tea.Cmd {
	// Repeated refresh presses must not restart an already running report.
	if m.loading && m.pending == r {
		return nil
	}
	forced := len(force) > 0 && force[0]
	if m.cancel != nil {
		m.cancel()
	}
	ctx, cancel := context.WithTimeout(m.ctx, 2*time.Minute)
	m.cancel = cancel
	m.loading = true
	m.loadingSince = time.Now()
	m.err = ""
	m.request++
	id := m.request
	o := m.o
	prices := m.prices
	m.pending = r
	fxCmd := m.refreshExchange()
	backendCmd := func() tea.Msg {
		defer cancel()
		var s Snapshot
		var e error
		if o.Demo {
			loc, _ := time.LoadLocation(o.TZ)
			s = demo(r, loc)
		} else {
			s, e = loadWithPrices(ctx, o, r, prices)
		}
		if e == nil {
			_ = writeSnapshotCache(o, r, s)
		}
		return loadedMsg{s, e, id, r}
	}
	// Copy the candidate before running commands; only Update owns the map.
	candidate, found := m.reports[r]
	reportCmd := func() tea.Msg {
		if !found || o.NoCache {
			candidate, _ = readSnapshotCache(o, r)
		}
		if !o.NoCache && !o.Demo && !candidate.Loaded.IsZero() && time.Since(candidate.Loaded) <= 7*24*time.Hour {
			if !forced && snapshotFresh(candidate, o.CacheTTL, time.Now()) {
				cancel()
				return reusedMsg{candidate, r, id}
			}
			return tea.Sequence(func() tea.Msg { return cachedMsg{candidate, r, id} }, backendCmd)()
		}
		return backendCmd()
	}
	return tea.Batch(m.spin.Tick, fxCmd, reportCmd, m.refreshPrices(forced))
}
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch v := msg.(type) {
	case string:
		if v == "initial-load" {
			return m, m.refresh(m.o.Range)
		}
	case priceTickMsg:
		return m, tea.Batch(m.refreshPrices(false), priceTick())
	case pricesMsg:
		m.priceLoading = false
		if v.err != nil {
			m.priceErr = v.err.Error()
			return m, nil
		}
		changed := v.catalog.revision() != m.o.priceRevision
		m.priceErr = ""
		m.prices = v.catalog
		m.o.priceRevision = v.catalog.revision()
		if !changed {
			if m.s.PriceRevision == m.o.priceRevision {
				m.s.PriceDate = v.catalog.Fetched
			}
			return m, nil
		}
		m.reports = nil
		if !m.loading {
			return m, m.refresh(m.o.Range, true)
		}
		return m, nil
	case tea.MouseMsg:
		if m.choosingTheme {
			return m, nil
		}
		if m.view == 0 && !m.activityDetail && !m.details && m.width >= 96 && m.height >= 32 && m.layout == 0 && v.Y >= 19 && v.Y < 16+(m.height-20)/2-1 && v.X >= 5 && v.X < (m.width-4)/2 {
			rows := m.chartPeriods()
			count := max(1, ((m.width-6)/2-6)/3)
			start := max(0, min(m.dayCursor, max(0, len(rows)-1))-count+1)
			m.dayCursor = min(max(0, len(rows)-1), start+(v.X-5)/3)
			m.widget = 0
		}
	case tea.WindowSizeMsg:
		m.width = v.Width
		m.height = v.Height
	case cachedMsg:
		if v.id == m.request && m.loading {
			m.s = v.s
			m.o.Range = v.r
			m.cached = true
		}
		return m, nil
	case reusedMsg:
		if v.id != m.request {
			return m, nil
		}
		m.loading = false
		m.cached = true
		m.s, m.o.Range = v.s, v.r
		if m.o.managedPrices && m.s.PriceRevision == m.o.priceRevision {
			m.s.PriceDate = m.prices.Fetched
		}
		m.cursor, m.details = 0, false
		m.remember(v.r, m.s)
		if m.o.managedPrices && m.s.PriceRevision != m.o.priceRevision {
			return m, m.refresh(v.r, true)
		}
		return m, nil
	case exportedMsg:
		if v.err != nil {
			m.notice = "Export failed: " + v.err.Error()
		} else {
			m.notice = "Saved " + v.path
		}
		return m, nil
	case exchangeMsg:
		if v.id != m.fxRequest {
			return m, nil
		}
		m.fxLoading = false
		if v.err != nil {
			m.fxErr = v.err.Error()
		} else {
			m.fx = v.exchange
			if m.exchanges == nil {
				m.exchanges = make(map[string]Exchange)
			}
			m.exchanges[v.exchange.Currency] = v.exchange
			m.fxErr = ""
		}
	case loadedMsg:
		if v.id != m.request {
			return m, nil
		}
		m.loading = false
		if v.err != nil {
			m.err = v.err.Error()
		} else {
			m.s = v.s
			m.remember(v.r, v.s)
			m.cached = false
			m.o.Range = v.r
			m.cursor = 0
			m.details = false
		}
		if v.err == nil && m.o.managedPrices && v.s.PriceRevision != m.o.priceRevision {
			return m, m.refresh(v.r, true)
		}
	case tea.KeyMsg:
		key := v.String()
		if key == "ctrl+c" {
			if m.cancel != nil {
				m.cancel()
			}
			if m.fxCancel != nil {
				m.fxCancel()
			}
			return m, tea.Quit
		}
		if m.choosingTheme {
			return m.updateThemePicker(v)
		}
		if m.exporting {
			if key == "esc" || key == "q" {
				m.exporting = false
				return m, nil
			}
			formats := map[string]string{"1": "json", "2": "csv", "3": "svg", "4": "png"}
			if format, ok := formats[key]; ok {
				m.exporting = false
				return m, m.exportCmd(format)
			}
			return m, nil
		}
		if m.editing != "" {
			if key == "esc" {
				m.editing = ""
				m.input.Blur()
				return m, nil
			}
			if key == "enter" {
				parts := strings.Fields(m.input.Value())
				if pair := strings.Split(strings.ReplaceAll(m.input.Value(), " to ", "→"), "→"); len(pair) == 2 {
					parts = []string{strings.TrimSpace(pair[0]), strings.TrimSpace(pair[1])}
				}
				loc, _ := time.LoadLocation(m.o.TZ)
				var r datefilter.Range
				var e error
				if len(parts) == 1 && parts[0] == "month" {
					r, e = datefilter.Resolve("", "", 0, m.o.Group, time.Now(), loc)
				} else if len(parts) == 2 && parts[0] == "last" {
					var n int
					if _, err := fmt.Sscanf(parts[1], "%d", &n); err != nil || n <= 0 || fmt.Sprint(n) != parts[1] {
						e = fmt.Errorf("use last N with a positive integer")
					} else {
						r, e = datefilter.Resolve("", "", n, m.o.Group, time.Now(), loc)
					}
				} else if len(parts) == 2 {
					s, u := m.canonicalDate(parts[0]), m.canonicalDate(parts[1])
					if s == "*" {
						s = ""
					}
					if u == "*" {
						u = ""
					}
					r, e = datefilter.Resolve(s, u, 0, m.o.Group, time.Now(), loc)
				} else {
					e = fmt.Errorf("enter two dates (use * for an open bound), month, or last N")
				}
				if e != nil {
					m.err = e.Error()
					return m, nil
				}
				m.editing = ""
				m.input.Blur()
				return m, m.refresh(r)
			}
			var cmd tea.Cmd
			m.input, cmd = m.input.Update(msg)
			return m, cmd
		}
		if key == "q" {
			if m.cancel != nil {
				m.cancel()
			}
			if m.fxCancel != nil {
				m.fxCancel()
			}
			return m, tea.Quit
		}
		if key == "esc" {
			m.details = false
			m.info = ""
			m.activityDetail = false
			m.help = false
			m.err = ""
			return m, nil
		}
		if key == "?" {
			m.help = !m.help
			m.helpOffset = 0
			return m, nil
		}
		if key == "ctrl+t" || key == "T" {
			cmd := m.openThemePicker()
			return m, cmd
		}
		if m.help {
			switch key {
			case "j", "down":
				m.helpOffset++
			case "k", "up":
				m.helpOffset--
			case "home":
				m.helpOffset = 0
			case "end":
				m.helpOffset = len(strings.Split(m.helpText(), "\n"))
			}
			m.helpOffset = max(0, min(m.helpOffset, max(0, len(strings.Split(m.helpText(), "\n"))-max(1, m.height-6))))
			return m, nil
		}
		switch key {
		case "1", "2", "3", "4", "5":
			m.view = int(key[0] - '1')
			m.activityDetail = false
			m.cursor = 0
			m.details = false
		case "tab":
			m.view = (m.view + 1) % len(views)
			m.activityDetail = false
			m.cursor = 0
			m.details = false
		case "shift+tab":
			m.view = (m.view + len(views) - 1) % len(views)
			m.cursor = 0
			m.details = false
		case "d":
			m.o.Group = "daily"
			m.savePreference(func(p *Preferences) { p.Grouping = m.o.Group })
			m.cursor = 0
		case "w":
			m.o.Group = "weekly"
			m.savePreference(func(p *Preferences) { p.Grouping = m.o.Group })
			m.cursor = 0
		case "m":
			m.o.Group = "monthly"
			m.savePreference(func(p *Preferences) { p.Grouping = m.o.Group })
			m.cursor = 0
		case "c":
			m.cost = !m.cost
			m.savePreference(func(p *Preferences) {
				p.Display = "tokens"
				if m.cost {
					p.Display = "cost"
				}
			})
		case "s":
			switch m.view {
			case 2:
				m.o.preferences.ModelsSort = cycle(modelSorts, m.tabSort())
				m.savePreference(func(p *Preferences) { p.ModelsSort = m.o.preferences.ModelsSort })
			case 4:
				m.o.preferences.SessionsSort = cycle(sessionSorts, m.tabSort())
				m.savePreference(func(p *Preferences) { p.SessionsSort = m.o.preferences.SessionsSort })
			default:
				m.sortMode = (m.sortMode + 1) % 3
			}
			m.cursor = 0
			return m, nil
		case "D":
			m.o.preferences.DateFormat = cycle([]string{"european", "us", "iso"}, m.o.preferences.DateFormat)
			m.savePreference(func(p *Preferences) { p.DateFormat = m.o.preferences.DateFormat })
			return m, nil
		case "H":
			m.o.preferences.ClockFormat = cycle([]string{"24h", "12h"}, m.o.preferences.ClockFormat)
			m.savePreference(func(p *Preferences) { p.ClockFormat = m.o.preferences.ClockFormat })
			return m, nil
		case "j", "down":
			if m.cursor < len(m.rows())-1 {
				m.cursor++
			}
		case "k", "up":
			if m.cursor > 0 {
				m.cursor--
			}
		case "home", "g":
			m.cursor = 0
		case "end", "G":
			m.cursor = max(0, len(m.rows())-1)
		case "n":
			m.compactNumbers = !m.compactNumbers
			m.savePreference(func(p *Preferences) { p.CompactNumbers = m.compactNumbers })
		case "e":
			m.o.Currency = cycle([]string{"USD", "EUR", "GBP", "JPY"}, m.o.Currency)
			m.savePreference(func(p *Preferences) { p.Currency = m.o.Currency })
			return m, m.refreshExchange()
		case "p":
			m.preset = (m.preset + 1) % 4
			m.notice = []string{"This calendar month", "This billing cycle", "Last 30 days", "Since August 1"}[m.preset]
			return m, m.refresh(m.presetRange(m.preset))
		case "b":
			m.showPlan = !m.showPlan
		case "o":
			m.exporting = true
		case "h":
			m.info = "Hourly / 5-hour costs unavailable: ccusage unified JSON has no timed usage events. Daily, weekly, monthly are available."
		case "left":
			m.dayCursor = max(0, m.dayCursor-1)
		case "right":
			m.dayCursor = min(max(0, len(m.chartPeriods())-1), m.dayCursor+1)
		case "[":
			m.widget = (m.widget + 3) % 4
		case "]":
			m.widget = (m.widget + 1) % 4
		case "v":
			m.layout = (m.layout + 1) % 2
			m.savePreference(func(p *Preferences) {
				p.Layout = "dashboard"
				if m.layout == 1 {
					p.Layout = "stacked"
				}
			})
		case "enter":
			if m.view == 0 && !m.activityDetail && m.width >= 96 && m.height >= 32 {
				selectedPeriod := ""
				periods := m.chartPeriods()
				if m.widget == 0 && len(periods) > 0 {
					selectedPeriod = periods[min(m.dayCursor, len(periods)-1)].Name
				}
				switch m.widget {
				case 0:
					m.activityDetail = true
				case 1:
					m.view = 2
				case 2:
					m.view = 4
				case 3:
					m.activityDetail = true
				}
				m.cursor = 0
				if selectedPeriod != "" {
					for i, r := range m.rows() {
						if r.Name == selectedPeriod {
							m.cursor = i
							break
						}
					}
				}
			} else {
				m.details = !m.details
			}
		case "a":
			m.agent = cycle(names(m.s, "agent"), m.agent)
			m.cursor = 0
			m.details = false
		case "f":
			m.modelFilter = cycle(names(m.s, "model"), m.modelFilter)
			m.cursor = 0
			m.details = false
		case "x":
			m.agent = ""
			m.modelFilter = ""
			m.cursor = 0
		case "r":
			r := m.o.Range
			if m.loading {
				r = m.pending
			}
			return m, m.refresh(r, true)
		case "t":
			m.editing = "range"
			m.input.SetValue(m.rangeInput())
			m.input.Focus()
			return m, textinput.Blink
		}
	}
	var cmd tea.Cmd
	if m.choosingTheme {
		m.themeQuery, cmd = m.themeQuery.Update(msg)
	}
	if m.loading {
		var spinCmd tea.Cmd
		m.spin, spinCmd = m.spin.Update(msg)
		cmd = tea.Batch(cmd, spinCmd)
	}
	return m, cmd
}
func cycle(ss []string, s string) string {
	for i, v := range ss {
		if s == v {
			return ss[(i+1)%len(ss)]
		}
	}
	return ""
}
func (m model) rows() []Row {
	raw := m.s.Sections[m.o.Group]
	base := filtered(raw, m.agent, m.modelFilter)
	var rows []Row
	switch m.view {
	case 0:
		rows = base
	case 1:
		rows = rank(raw, "agents", m.agent, m.modelFilter)
	case 2:
		rows = rank(filtered(raw, m.agent, ""), "models", "", m.modelFilter)
	case 3:
		u := total(base)
		rows = []Row{{Name: "Input", Usage: Usage{Tokens: u.Input}}, {Name: "Output", Usage: Usage{Tokens: u.Output}}, {Name: "Cache read", Usage: Usage{Tokens: u.Read}}, {Name: "Cache write", Usage: Usage{Tokens: u.Write}}}
	case 4:
		rows = filtered(m.s.Sections["session"], m.agent, m.modelFilter)
	}
	rows = append([]Row(nil), rows...)
	if m.view == 2 || m.view == 4 {
		sortRows(rows, m.tabSort())
		return rows
	}
	sort.SliceStable(rows, func(i, j int) bool {
		if m.sortMode == 2 {
			return rows[i].Name < rows[j].Name
		}
		a, b := m.value(rows[i]), m.value(rows[j])
		if a.Value == b.Value {
			return rows[i].Name < rows[j].Name
		}
		if m.sortMode == 1 {
			return a.Value < b.Value
		}
		return a.Value > b.Value
	})
	return rows
}
func (m model) value(r Row) Metric {
	if m.cost && m.view != 3 {
		return r.Usage.Cost
	}
	return r.Usage.Tokens
}
func number(v float64) string {
	s := fmt.Sprintf("%.0f", v)
	for i := len(s) - 3; i > 0; i -= 3 {
		s = s[:i] + "," + s[i:]
	}
	return s
}
func format(v Metric, cost bool) string {
	if !v.Known {
		return "unavailable"
	}
	s := number(v.Value)
	if cost {
		s = fmt.Sprintf("$%.4f", v.Value)
	}
	if v.Partial {
		s += " + ?"
	}
	return s
}
func safe(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || r == 0x1b {
			return -1
		}
		return r
	}, s)
}
func color(name string) lipgloss.Color { return colorFor(name, activeTheme) }
func colorFor(name, theme string) lipgloss.Color {
	h := fnv.New32a()
	_, _ = h.Write([]byte(name))
	p := paletteFor(theme).series
	return lipgloss.Color(p[int(h.Sum32())%len(p)])
}
func clip(s string, w int) string { return ansi.Truncate(s, max(1, w), "…") }
func (m model) compactView() string {
	if m.width < 50 || m.height < 16 {
		return "Tokenlens\n\nResize to at least 50 × 16 for the dashboard.\nq quit"
	}
	w := m.width - 4
	headerExtra := 0
	var b strings.Builder
	badge := "LOCAL"
	if m.o.Demo {
		badge = "DEMO · SYNTHETIC"
	}
	b.WriteString(bright.Render("◈  TOKENLENS") + "  " + muted.Render("/  usage, in focus") + "   " + accent.Render(badge) + "\n")
	b.WriteString(muted.Render(clip(m.formatRange(m.o.Range)+"  ·  "+m.o.TZ, w)) + "\n" + m.themeIndicator() + "\n")
	u := total(filtered(m.s.Sections["daily"], m.agent, m.modelFilter))
	summary := "Tokens  " + bright.Render(format(u.Tokens, false)) + "    Estimated cost (" + m.fx.Currency + ")  " + accent.Render(m.fx.format(u.Cost))
	if m.s.Loaded.IsZero() {
		summary = "Waiting for your first snapshot"
	}
	if ansi.StringWidth(summary) > w && !m.s.Loaded.IsZero() {
		b.WriteString("Tokens  " + bright.Render(format(u.Tokens, false)) + "\n" + "Estimated cost (" + m.fx.Currency + ")  " + accent.Render(m.fx.format(u.Cost)) + "\n")
		headerExtra++
	} else {
		b.WriteString(clip(summary, w) + "\n")
	}
	b.WriteString(muted.Render(clip(m.pricingStatus(), w)) + "\n")
	headerExtra++
	fresh := "not loaded"
	if !m.s.Loaded.IsZero() {
		fresh = "snapshot " + m.formatTimestamp(m.s.Loaded)
		if m.cached {
			fresh = "cached " + fresh
		}
	}
	if m.loading {
		fresh = m.spin.View() + " loading " + m.formatRange(m.pending) + " · " + fresh
	}
	b.WriteString(muted.Render(clip(fresh, w)) + "\n")
	if m.o.Currency != "USD" {
		status := muted.Width(w).Render(m.exchangeStatus())
		b.WriteString(status + "\n")
		headerExtra += lipgloss.Height(status)
	}
	b.WriteString("\n")
	tabs := []string{}
	for i, v := range views {
		s := fmt.Sprintf("%d %s", i+1, v)
		if i == m.view {
			s = accent.Bold(true).Render(s)
		} else {
			s = muted.Render(s)
		}
		tabs = append(tabs, s)
	}
	if w < 88 {
		b.WriteString(accent.Bold(true).Render(fmt.Sprintf("%d / %s", m.view+1, views[m.view])) + muted.Render("    tab switch view"))
	} else {
		b.WriteString(strings.Join(tabs, "   "))
	}
	b.WriteString("\n")
	a, f := m.agent, m.modelFilter
	if a == "" {
		a = "all"
	}
	if f == "" {
		f = "all"
	}
	filterLabel := "Agent: " + safe(a) + "   Model: " + safe(f) + "   Group: " + m.o.Group
	if m.view == 2 || m.view == 4 {
		filterLabel = "[s] " + m.sortLabel() + " · " + filterLabel
	}
	b.WriteString(muted.Render(clip(filterLabel, w)) + "\n")
	b.WriteString(muted.Render(strings.Repeat("─", w)) + "\n")
	if m.info != "" {
		b.WriteString("DATA AVAILABILITY\n\n" + m.info + "\n\nesc close")
	} else if m.exporting {
		b.WriteString("EXPORT FILTERED VIEW\n\n1 JSON    2 CSV    3 SVG    4 PNG\n\nesc cancel")
	} else if m.help {
		b.WriteString(m.helpText())
	} else if m.editing != "" {
		b.WriteString(bright.Render("Change date range") + "\n\n" + m.input.View() + "\n\n" + muted.Render(m.rangeHelp()+"\nenter apply · esc cancel"))
		if m.err != "" {
			b.WriteString("\n\n" + safe(m.err))
		}
	} else if m.err != "" {
		b.WriteString(bright.Render("Could not refresh") + "\n\n" + lipgloss.NewStyle().Width(w).Render(safe(m.err)) + "\n\n" + muted.Render("r retry · t change range · esc return to previous snapshot"))
	} else if m.details {
		rows := m.rows()
		if len(rows) > 0 {
			r := rows[min(m.cursor, len(rows)-1)]
			b.WriteString(bright.Render(clip(safe(m.rowLabel(r)), w)) + "\n" + m.sessionTimes(r) + "\n")
			for _, v := range []struct {
				label string
				v     Metric
				c     bool
			}{{"Total tokens", r.Usage.Tokens, false}, {"Estimated cost · " + m.fx.Currency, r.Usage.Cost, true}, {"Input", r.Usage.Input, false}, {"Output", r.Usage.Output, false}, {"Cache read", r.Usage.Read, false}, {"Cache write", r.Usage.Write, false}} {
				b.WriteString(fmt.Sprintf("%-24s %s\n", v.label, m.formatMetric(v.v, v.c)))
			}
			if len(r.Models) > 0 {
				ss := []string{}
				for _, v := range r.Models {
					ss = append(ss, safe(v.Name))
				}
				b.WriteString("\n" + muted.Render(clip("Models: "+strings.Join(ss, ", "), w)))
			}
			if len(r.Metadata) > 0 {
				keys := []string{}
				for k := range r.Metadata {
					keys = append(keys, k)
				}
				sort.Strings(keys)
				for _, k := range keys {
					b.WriteString("\n" + muted.Render(clip(safe(k+": "+m.metadataValue(r, k)), w)))
				}
			}
			b.WriteString("\n\n" + muted.Render("esc / enter close · ↑ ↓ inspect another row"))
		}
	} else {
		rows := m.rows()
		metricLabel := "TOKENS"
		if m.cost && m.view != 3 {
			metricLabel = "EST. " + m.fx.Currency
		}
		order := m.sortLabel()
		heading := views[m.view]
		if m.view == 0 {
			heading = "Activity · " + m.o.Group
		}
		b.WriteString(bright.Render(heading) + muted.Render("  /  "+metricLabel+" · "+order) + "\n")
		if m.view == 0 {
			contributors := rank(m.s.Sections["daily"], "agents", m.agent, m.modelFilter)
			sort.Slice(contributors, func(i, j int) bool { return contributors[i].Usage.Cost.Value > contributors[j].Usage.Cost.Value })
			if len(contributors) > 0 {
				c := contributors[0]
				share := ""
				if u.Cost.Known && u.Cost.Value > 0 && !u.Cost.Partial && !c.Usage.Cost.Partial {
					share = fmt.Sprintf(" · %.1f%%", 100*c.Usage.Cost.Value/u.Cost.Value)
				}
				b.WriteString(muted.Render(clip("Biggest estimated cost: "+safe(c.Name)+" · "+m.fx.format(c.Usage.Cost)+share, w)) + "\n")
			} else {
				b.WriteString(muted.Render("Agent cost breakdown unavailable") + "\n")
			}
		}
		if m.view == 3 {
			b.WriteString(muted.Render(clip("Reported token categories; category pricing unavailable.", w)) + "\n")
		}
		if len(rows) == 0 {
			b.WriteString("\n" + bright.Render("No usage in this view") + "\n" + muted.Render("t change range · x clear filters · r refresh · --demo to explore"))
		} else {
			sum, maxV := Metric{}, 0.0
			for _, r := range rows {
				v := m.value(r)
				sum.add(v)
				maxV = max(maxV, v.Value)
			}
			slots := max(1, m.height-18-headerExtra)
			start := max(0, m.cursor-slots+1)
			end := min(len(rows), start+slots)
			for i := start; i < end; i++ {
				r := rows[i]
				v := m.value(r)
				labelWidth := min(32, max(12, w/3))
				barWidth := max(2, w-labelWidth-34)
				n := 0
				if maxV > 0 && v.Known {
					n = int(v.Value / maxV * float64(barWidth))
				}
				share := "    —"
				if sum.Known && !sum.Partial && sum.Value > 0 && v.Known && !v.Partial {
					share = fmt.Sprintf("%5.1f%%", 100*v.Value/sum.Value)
				}
				pointer := "  "
				if i == m.cursor {
					pointer = accent.Render("› ")
				}
				name := clip(safe(m.rowLabel(r)), labelWidth)
				name += strings.Repeat(" ", max(0, labelWidth-ansi.StringWidth(name)))
				c := color(r.Agent)
				if r.Agent == "" {
					c = color(r.Name)
				}
				if r.Agent == "all" {
					c = lipgloss.Color(paletteFor(activeTheme).accent)
				}
				bar := lipgloss.NewStyle().Foreground(c).Render(strings.Repeat("━", n)) + muted.Render(strings.Repeat("·", barWidth-n))
				b.WriteString(clip(pointer+name+"  "+bar+fmt.Sprintf("  %15s  %s", m.formatMetric(v, m.cost && m.view != 3), share), w) + "\n")
			}
			b.WriteString(muted.Render(fmt.Sprintf("\n%d–%d of %d  ·  enter details", start+1, end, len(rows))))
			if sum.Partial {
				b.WriteString(muted.Render(" · + ? = incomplete metrics"))
			}
		}
	}
	footer := muted.Render("s sort  D date  H clock  ? help  q quit")
	content := b.String()
	lines := strings.Split(content, "\n")
	maxLines := m.height - 4
	if len(lines) > maxLines {
		lines = lines[:maxLines]
		lines[maxLines-1] = muted.Render("… resize terminal to see more")
	}
	for i := range lines {
		lines[i] = clip(lines[i], w)
	}
	content = strings.Join(lines, "\n")
	content += strings.Repeat("\n", max(1, maxLines-len(lines)+1)) + clip(footer, w)
	return themeRender(lipgloss.NewStyle().Foreground(ink).Padding(1, 2).Render(content), m.o.Theme, m.width, m.height)
}

func (m *model) refreshExchange() tea.Cmd {
	return m.refreshExchangeAt(time.Now())
}

func (m *model) refreshExchangeAt(now time.Time) tea.Cmd {
	if m.fxLoading && m.fxTarget == m.o.Currency {
		return nil
	}
	if m.fxCancel != nil {
		m.fxCancel()
	}
	m.fxRequest++
	m.fxLoading = false
	m.fxTarget = m.o.Currency
	if m.fx.Currency != m.o.Currency {
		if m.fx.available() {
			if m.exchanges == nil {
				m.exchanges = make(map[string]Exchange)
			}
			m.exchanges[m.fx.Currency] = m.fx
		}
		m.fx = initialExchange(m.o, now)
		m.fxErr = ""
		if x, ok := m.exchanges[m.o.Currency]; ok && x.FetchedAt.After(m.fx.FetchedAt) {
			m.fx = x
		}
	}
	if m.o.Currency == "USD" {
		m.fx = usdExchange()
		m.fxErr = ""
		return nil
	}
	if m.fx.fresh(now) {
		return nil
	}
	m.fxLoading = true
	m.fxErr = ""
	ctx, cancel := context.WithTimeout(m.ctx, 10*time.Second)
	m.fxCancel = cancel
	o, id := m.o, m.fxRequest
	return func() tea.Msg {
		defer cancel()
		if o.Demo {
			return exchangeMsg{exchange: Exchange{Currency: o.Currency, Rate: 0.9, Date: "sample", Source: "synthetic demo rate", FetchedAt: time.Now()}, id: id}
		}
		x, err := fetchAndCacheExchange(ctx, &http.Client{Timeout: 10 * time.Second}, exchangeEndpoint, o)
		return exchangeMsg{exchange: x, err: err, id: id}
	}
}
func (m model) formatMetric(v Metric, cost bool) string {
	if cost {
		return m.fx.format(v)
	}
	if m.compactNumbers && v.Known {
		suffix := ""
		number := v.Value
		if number >= 1e9 {
			number /= 1e9
			suffix = "B"
		} else if number >= 1e6 {
			number /= 1e6
			suffix = "M"
		} else if number >= 1e3 {
			number /= 1e3
			suffix = "k"
		}
		if suffix != "" {
			s := fmt.Sprintf("%.2f%s", number, suffix)
			if v.Partial {
				s += " + ?"
			}
			return s
		}
	}
	return format(v, false)
}
func (m model) exchangeStatus() string {
	if !m.fx.available() {
		if m.fxErr != "" {
			return "FX  unavailable for " + m.o.Currency + " · costs unavailable · r retry"
		}
		return "FX  loading " + m.o.Currency + " exchange rate…"
	}
	s := m.exchangeLabel()
	if m.fxErr != "" {
		s += " · refresh failed; previous rate"
	} else if m.fxLoading {
		s += " · refreshing"
	}
	return s
}
