//go:build !windows

package app

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// Exercise Sequence/Batch delivery and backend execution through the real loop.
type reportFlowModel struct {
	model
	force     bool
	sawCached bool
}

func (m reportFlowModel) Init() tea.Cmd { return func() tea.Msg { return "test-load" } }
func (m reportFlowModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if msg == "test-load" {
		return m, m.refresh(m.o.Range, m.force)
	}
	if _, ok := msg.(cachedMsg); ok {
		m.sawCached = true
	}
	next, cmd := m.model.Update(msg)
	m.model = next.(model)
	switch msg.(type) {
	case loadedMsg, reusedMsg:
		return m, tea.Quit
	}
	return m, cmd
}
func TestReportLoadFlow(t *testing.T) {
	for _, tc := range []struct {
		name                              string
		age                               time.Duration
		force, noCache, fail, wantBackend bool
	}{
		{name: "fresh"},
		{name: "forced", force: true, wantBackend: true},
		{name: "stale", age: 10 * time.Minute, wantBackend: true},
		{name: "disabled", noCache: true, wantBackend: true},
		{name: "failed refresh retains cached report", age: 10 * time.Minute, fail: true, wantBackend: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			marker := filepath.Join(dir, "called")
			backend := filepath.Join(dir, "backend")
			script := "#!/bin/sh\nprintf called > \"$TOKENLENS_TEST_MARKER\"\nprintf '%s' '{\"daily\":[],\"weekly\":[],\"monthly\":[],\"session\":[]}'\n"
			if tc.fail {
				script += "exit 1\n"
			}
			if err := os.WriteFile(backend, []byte(script), 0700); err != nil {
				t.Fatal(err)
			}
			t.Setenv("TOKENLENS_TEST_MARKER", marker)
			o, err := options([]string{"--currency", "USD", "--ccusage", backend, "--cache-dir", dir}, time.Now())
			if err != nil {
				t.Fatal(err)
			}
			cached := Snapshot{Loaded: time.Now().Add(-tc.age), Sections: map[string][]Row{"daily": {{Name: "cached"}}}}
			if err = writeSnapshotCache(o, o.Range, cached); err != nil {
				t.Fatal(err)
			}
			o.NoCache = tc.noCache
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			p := tea.NewProgram(reportFlowModel{model: newModel(ctx, o), force: tc.force}, tea.WithContext(ctx), tea.WithInput(nil), tea.WithOutput(io.Discard), tea.WithoutRenderer(), tea.WithoutSignalHandler())
			result, err := p.Run()
			if err != nil {
				t.Fatal(err)
			}
			final := result.(reportFlowModel)
			_, err = os.Stat(marker)
			if called := err == nil; called != tc.wantBackend {
				t.Fatalf("backend called=%v, want %v", called, tc.wantBackend)
			}
			if final.loading {
				t.Fatal("load did not finish")
			}
			if final.sawCached != (tc.wantBackend && !tc.noCache) {
				t.Fatal("wrong stale-while-refreshing behavior")
			}
			if tc.fail && (final.err == "" || !final.cached || final.s.Sections["daily"][0].Name != "cached") {
				t.Fatal("failed refresh lost cached data or error")
			}
			if !tc.fail && tc.wantBackend && final.cached {
				t.Fatal("fresh backend data labeled cached")
			}
		})
	}
}
