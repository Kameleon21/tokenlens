package app

import (
	"encoding/csv"
	"encoding/json"
	"image/png"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"
)

func TestBackendAutoLaunch(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix executable fixture")
	}
	dir := t.TempDir()
	t.Setenv("PATH", dir)
	if _, _, e := backendCommand("ccusage"); e == nil {
		t.Fatal("missing backend accepted")
	}
	bunx := filepath.Join(dir, "bunx")
	if e := os.WriteFile(bunx, []byte("#!/bin/sh\nexit 0\n"), 0755); e != nil {
		t.Fatal(e)
	}
	path, args, e := backendCommand("ccusage")
	if e != nil || path != bunx || strings.Join(args, " ") != "--bun ccusage@20.0.20" {
		t.Fatalf("%s %v %v", path, args, e)
	}
	if _, _, e = backendCommand("/custom/missing/ccusage"); e == nil {
		t.Fatal("explicit path silently replaced")
	}
	cc := filepath.Join(dir, "ccusage")
	_ = os.WriteFile(cc, []byte("#!/bin/sh\nexit 0\n"), 0755)
	path, args, e = backendCommand("ccusage")
	if e != nil || path != cc || len(args) != 0 {
		t.Fatal("installed backend not preferred")
	}
}
func TestBillingCycleClamp(t *testing.T) {
	for _, c := range []struct {
		now, want string
		day       int
	}{{"2026-03-30", "2026-02-28", 31}, {"2024-03-01", "2024-02-29", 31}, {"2026-01-01", "2025-12-15", 15}, {"2026-03-31", "2026-03-31", 31}} {
		now, _ := time.Parse("2006-01-02", c.now)
		if got := billingStart(now, c.day).Format("2006-01-02"); got != c.want {
			t.Fatalf("%+v: %s", c, got)
		}
	}
}
func TestCustomizationOptions(t *testing.T) {
	for _, args := range [][]string{{"--theme", "bad"}, {"--plan-cost", "NaN"}, {"--plan-cost", "-1"}, {"--billing-day", "0"}, {"--billing-day", "32"}} {
		if _, e := options(args, time.Now()); e == nil {
			t.Fatal("accepted", args)
		}
	}
}
func TestAllThemesFit(t *testing.T) {
	original := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(original)
	defer applyTheme("dark")
	m := fixtureModel()
	m.o.Currency = "EUR"
	m.fx = Exchange{Currency: "EUR", Rate: .86, Date: "2026-09-04", Source: "ECB via Frankfurter"}
	for _, theme := range []string{"dark", "light", "ascii"} {
		applyTheme(theme)
		m.o.Theme = theme
		for _, wh := range [][2]int{{50, 16}, {80, 24}, {96, 32}, {160, 50}, {300, 80}} {
			m.width, m.height = wh[0], wh[1]
			for view := 0; view < 5; view++ {
				m.view = view
				s := ansi.Strip(m.View())
				if len(strings.Split(s, "\n")) > m.height {
					t.Fatalf("height %s %v", theme, wh)
				}
				for _, line := range strings.Split(s, "\n") {
					if ansi.StringWidth(line) > m.width {
						t.Fatalf("width %s %v", theme, wh)
					}
				}
				if theme == "ascii" && strings.ContainsAny(s, "╭╮╰╯│█▓▒░€") {
					t.Fatal("non ASCII chrome")
				}
			}
		}
	}
}
func TestSelectedDayOpensExactPeriod(t *testing.T) {
	m := fixtureModel()
	m.width = 160
	m.height = 50
	m.dayCursor = 3
	m.widget = 0
	want := m.chartPeriods()[3].Name
	v, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	n := v.(model)
	if !n.activityDetail || n.rows()[n.cursor].Name != want {
		t.Fatalf("opened wrong day %s", want)
	}
}
func TestFilteredExports(t *testing.T) {
	m := fixtureModel()
	m.o.ExportDir = t.TempDir()
	m.agent = "codex"
	m.o.Currency = "EUR"
	m.fx = Exchange{Currency: "EUR", Rate: .86, Date: "2026-09-04", Source: "ECB via Frankfurter"}
	for _, kind := range []string{"json", "csv", "svg", "png"} {
		path, e := m.writeExport(kind)
		if e != nil {
			t.Fatal(kind, e)
		}
		b, e := os.ReadFile(path)
		if e != nil {
			t.Fatal(e)
		}
		switch kind {
		case "json":
			var out struct {
				Currency string `json:"currency"`
				Rows     []struct {
					Agent string `json:"agent"`
					USD   struct {
						Value float64 `json:"value"`
					} `json:"estimated_cost_usd"`
					Cost struct {
						Value float64 `json:"value"`
					} `json:"estimated_cost"`
				} `json:"rows"`
			}
			if e = json.Unmarshal(b, &out); e != nil {
				t.Fatal(e)
			}
			if out.Currency != "EUR" || len(out.Rows) != 30 {
				t.Fatal("wrong scope")
			}
			for _, r := range out.Rows {
				if r.Agent != "codex" || r.Cost.Value != r.USD.Value*.86 {
					t.Fatal("wrong filter or rate")
				}
			}
		case "csv":
			rows, e := csv.NewReader(strings.NewReader(string(b))).ReadAll()
			if e != nil || len(rows) != 31 {
				t.Fatal("invalid CSV", e)
			}
		case "svg":
			if !strings.Contains(string(b), "<svg") || !strings.Contains(string(b), "codex") {
				t.Fatal("invalid SVG")
			}
		case "png":
			f, e := os.Open(path)
			if e != nil {
				t.Fatal(e)
			}
			_, e = png.Decode(f)
			_ = f.Close()
			if e != nil {
				t.Fatal(e)
			}
		}
	}
}
func TestExportLabelsEscaped(t *testing.T) {
	m := fixtureModel()
	svg := m.exportSVG([]Row{{Name: "<script>alert(1)</script>", Usage: Usage{Cost: known(1)}}})
	if strings.Contains(svg, "<script>") {
		t.Fatal("unescaped SVG")
	}
	if csvSafe("=1+1") != "'=1+1" {
		t.Fatal("unsafe CSV text")
	}
}
