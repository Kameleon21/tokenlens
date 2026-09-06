package app

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"
)

func TestThemeSwitching(t *testing.T) {
	original := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(original)
	defer applyTheme("dark")
	m := fixtureModel()
	m.o.Theme = "dark"
	m.agent = "codex"
	m.view = 2
	for _, want := range append(append([]string{}, themeNames[1:]...), "dark") {
		next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'T'}})
		m = next.(model)
		if cmd != nil {
			t.Fatal("theme switch started external work")
		}
		if m.o.Theme != want || activeTheme != want {
			t.Fatalf("wanted %s, got %s / %s", want, m.o.Theme, activeTheme)
		}
		if m.agent != "codex" || m.view != 2 {
			t.Fatal("theme switch reset navigation or filters")
		}
		if !strings.Contains(m.View(), themeLabel(want)) {
			t.Fatalf("missing active theme label: %s", want)
		}
		if m.spin.Style.GetForeground() != accent.GetForeground() {
			t.Fatal("spinner kept previous theme")
		}
	}
}

func TestThemeStartupAndBackground(t *testing.T) {
	original := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(original)
	defer applyTheme("dark")
	for _, name := range themeNames {
		o, err := options([]string{"--theme", name}, time.Now())
		if err != nil || o.Theme != name {
			t.Fatalf("%s: %v", name, err)
		}
		if name == "ascii" {
			if strings.Contains(themeRender("hello", name, 50, 16), "\x1b") {
				t.Fatal("ASCII includes color codes")
			}
			continue
		}
		p, ok := themePalettes[name]
		if !ok {
			t.Fatalf("missing palette %s", name)
		}
		expected := lipgloss.NewStyle().Foreground(lipgloss.Color(p.foreground)).Background(lipgloss.Color(p.background)).Render("x")
		prefix := strings.SplitN(expected, "x", 2)[0]
		if prefix == "" || !strings.HasPrefix(themeRender("hello", name, 50, 16), prefix) {
			t.Fatalf("wrong background for %s", name)
		}
	}
}

func TestThemeIndicatorVisibleAtTop(t *testing.T) {
	defer applyTheme("dark")
	m := fixtureModel()
	for _, name := range themeNames {
		m.o.Theme = name
		applyTheme(name)
		for _, wh := range [][2]int{{50, 16}, {80, 24}, {96, 32}, {160, 50}} {
			m.width, m.height = wh[0], wh[1]
			for view := 0; view < 5; view++ {
				m.view = view
				lines := strings.Split(ansi.Strip(m.View()), "\n")
				top := strings.Join(lines[:min(4, len(lines))], "\n")
				if !strings.Contains(top, "Theme: "+themeLabel(name)) || !strings.Contains(top, "Shift+T next") {
					t.Fatalf("missing visible theme indicator for %s at %v view %d: %s", name, wh, view, top)
				}
			}
		}
	}
}
