package app

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
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
