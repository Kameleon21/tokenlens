package app

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/Kameleon21/tokenlens/internal/datefilter"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"time"
)

// Presence is preserved: missing metrics must never silently become zero.
type Metric struct {
	Value   float64
	Known   bool
	Partial bool
}

func (m *Metric) add(n Metric) {
	if !n.Known {
		m.Partial = true
		return
	}
	m.Value += n.Value
	m.Known = true
	m.Partial = m.Partial || n.Partial
}

type Usage struct{ Input, Output, Read, Write, Tokens, Cost Metric }

func (u *Usage) add(v Usage) {
	u.Input.add(v.Input)
	u.Output.add(v.Output)
	u.Read.add(v.Read)
	u.Write.add(v.Write)
	u.Tokens.add(v.Tokens)
	u.Cost.add(v.Cost)
}

type Row struct {
	// Derived once when reading a backend report or disk cache; never during sorting/rendering.
	firstActivity, lastActivity time.Time
	metadataTimes               map[string]time.Time
	Name, Agent                 string
	Usage                       Usage
	Models, Agents              []Row
	Metadata                    map[string]json.RawMessage
}
type Snapshot struct {
	PriceDate     time.Time
	PriceSource   string
	PriceRevision string
	Unpriced      []string
	Sections      map[string][]Row
	Loaded        time.Time
}

func metric(m map[string]json.RawMessage, keys ...string) Metric {
	for _, k := range keys {
		if v, ok := m[k]; ok && string(v) != "null" {
			var n float64
			if json.Unmarshal(v, &n) == nil && n >= 0 {
				return Metric{Value: n, Known: true}
			}
		}
	}
	return Metric{}
}
func parseRow(raw json.RawMessage) (Row, error) {
	var m map[string]json.RawMessage
	if e := json.Unmarshal(raw, &m); e != nil {
		return Row{}, e
	}
	if m == nil {
		return Row{}, fmt.Errorf("row must be an object")
	}
	str := func(keys ...string) string {
		for _, k := range keys {
			var s string
			if json.Unmarshal(m[k], &s) == nil && s != "" {
				return s
			}
		}
		return ""
	}
	r := Row{Name: str("period", "modelName"), Agent: str("agent")}
	r.Usage = Usage{metric(m, "inputTokens"), metric(m, "outputTokens"), metric(m, "cacheReadTokens"), metric(m, "cacheCreationTokens"), metric(m, "totalTokens"), metric(m, "totalCost", "cost")}
	if !r.Usage.Tokens.Known && r.Usage.Input.Known && r.Usage.Output.Known && r.Usage.Read.Known && r.Usage.Write.Known {
		r.Usage.Tokens = Metric{Value: r.Usage.Input.Value + r.Usage.Output.Value + r.Usage.Read.Value + r.Usage.Write.Value, Known: true}
	}
	for _, k := range []string{"modelBreakdowns", "agents"} {
		if v, ok := m[k]; ok {
			var list []json.RawMessage
			if e := json.Unmarshal(v, &list); e != nil {
				return r, e
			}
			for _, v := range list {
				child, e := parseRow(v)
				if e != nil {
					return r, e
				}
				if k == "agents" {
					child.Name = child.Agent
					r.Agents = append(r.Agents, child)
				} else {
					r.Models = append(r.Models, child)
				}
			}
		}
	}
	_ = json.Unmarshal(m["metadata"], &r.Metadata)
	r.prepareTimes()
	return r, nil
}
func parseSnapshot(b []byte) (Snapshot, error) {
	var raw map[string]json.RawMessage
	if e := json.Unmarshal(b, &raw); e != nil {
		return Snapshot{}, fmt.Errorf("ccusage returned invalid JSON: %w", e)
	}
	s := Snapshot{Sections: map[string][]Row{}, Loaded: time.Now()}
	for _, section := range []string{"daily", "weekly", "monthly", "session"} {
		v, ok := raw[section]
		if !ok {
			return s, fmt.Errorf("missing %s section; Tokenlens requires ccusage unified --sections and --by-agent support (verified with 20.0.20)", section)
		}
		var rows []json.RawMessage
		if e := json.Unmarshal(v, &rows); e != nil {
			return s, fmt.Errorf("invalid %s section: %w", section, e)
		}
		for _, v := range rows {
			r, e := parseRow(v)
			if e != nil {
				return s, e
			}
			if r.Name == "" {
				return s, fmt.Errorf("unsupported %s row: missing period", section)
			}
			s.Sections[section] = append(s.Sections[section], r)
		}
	}
	return s, nil
}
func backendArgs(r datefilter.Range, tz string) []string {
	a := []string{"daily", "--sections", "daily,weekly,monthly,session", "--by-agent", "--json", "--timezone", tz}
	if r.Since != "" {
		a = append(a, "--since", r.Since)
	}
	if r.Until != "" {
		a = append(a, "--until", r.Until)
	}
	return a
}
func load(ctx context.Context, bin string, r datefilter.Range, tz string, offline ...bool) (Snapshot, error) {
	return loadConfigured(ctx, bin, r, tz, len(offline) > 0 && offline[0], nil)
}
func loadWithPrices(ctx context.Context, o Options, r datefilter.Range, p priceCatalog) (Snapshot, error) {
	if !o.managedPrices {
		return load(ctx, o.Bin, r, o.TZ, o.Offline)
	}
	config, cleanup, err := p.configFile()
	if err != nil {
		return Snapshot{}, err
	}
	defer cleanup()
	s, err := loadConfigured(ctx, o.Bin, r, o.TZ, true, []string{"--config", config})
	if err == nil {
		p.applyCoverage(&s)
	}
	return s, err
}
func loadConfigured(ctx context.Context, bin string, r datefilter.Range, tz string, offline bool, extra []string) (Snapshot, error) {
	path, prefix, e := backendCommand(bin)
	if e != nil {
		return Snapshot{}, e
	}
	args := backendArgs(r, tz)
	if offline {
		args = append(args, "--offline")
	}
	args = append(args, extra...)
	cmd := exec.CommandContext(ctx, path, append(prefix, args...)...)
	configureBackend(cmd)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	b, e := cmd.Output()
	if e != nil {
		if ctx.Err() != nil {
			return Snapshot{}, fmt.Errorf("ccusage load interrupted or timed out: %w", ctx.Err())
		}
		msg := stderr.String()
		if len(msg) > 1500 {
			msg = msg[:1500]
		}
		return Snapshot{}, fmt.Errorf("ccusage failed: %v\n%s", e, msg)
	}
	return parseSnapshot(b)
}
func filtered(rows []Row, agent, model string) []Row {
	out := []Row{}
	for _, r := range rows {
		if agent != "" {
			if len(r.Agents) > 0 {
				found := false
				for _, a := range r.Agents {
					if a.Agent == agent {
						a.Name = r.Name
						a.Metadata = r.Metadata
						a.firstActivity, a.lastActivity, a.metadataTimes = r.firstActivity, r.lastActivity, r.metadataTimes
						r = a
						found = true
						break
					}
				}
				if !found {
					continue
				}
			} else if r.Agent != agent {
				continue
			}
		}
		if model != "" {
			u := Usage{}
			matches := []Row{}
			for _, m := range r.Models {
				if m.Name == model {
					u.add(m.Usage)
					matches = append(matches, m)
				}
			}
			if len(matches) == 0 {
				continue
			}
			r.Usage = u
			r.Models = matches
			r.Agents = nil
		}
		out = append(out, r)
	}
	return out
}
func total(rows []Row) Usage {
	u := Usage{}
	for _, r := range rows {
		u.add(r.Usage)
	}
	return u
}
func rank(rows []Row, kind, agent, model string) []Row {
	by := map[string]Row{}
	for _, r := range rows {
		children := r.Models
		if kind == "agents" {
			children = r.Agents
			if len(children) == 0 && r.Agent != "" && r.Agent != "all" {
				children = []Row{r}
			}
		}
		for _, c := range children {
			if kind == "agents" {
				if agent != "" && c.Agent != agent {
					continue
				}
				c.Name = c.Agent
				if model != "" {
					f := filtered([]Row{c}, "", model)
					if len(f) == 0 {
						continue
					}
					c = f[0]
				}
			} else if model != "" && c.Name != model {
				continue
			}
			x := by[c.Name]
			x.Name = c.Name
			x.Agent = c.Agent
			x.Usage.add(c.Usage)
			by[c.Name] = x
		}
	}
	out := []Row{}
	for _, r := range by {
		out = append(out, r)
	}
	return out
}
func names(s Snapshot, kind string) []string {
	set := map[string]bool{}
	for _, r := range s.Sections["daily"] {
		if kind == "agent" {
			for _, a := range r.Agents {
				set[a.Agent] = true
			}
			if r.Agent != "" && r.Agent != "all" {
				set[r.Agent] = true
			}
		} else {
			for _, m := range r.Models {
				set[m.Name] = true
			}
		}
	}
	out := []string{""}
	for n := range set {
		out = append(out, n)
	}
	sort.Strings(out[1:])
	return out
}

func bundledBackend(executable string) string {
	real, err := filepath.EvalSymlinks(executable)
	if err != nil {
		return ""
	}
	name := "ccusage"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	path := filepath.Join(filepath.Dir(real), "libexec", name)
	if resolved, err := exec.LookPath(path); err == nil {
		return resolved
	}
	return ""
}
func backendCommand(bin string) (string, []string, error) {
	if bin == "ccusage" {
		if executable, err := os.Executable(); err == nil {
			if path := bundledBackend(executable); path != "" {
				return path, nil, nil
			}
		}
	}
	if path, err := exec.LookPath(bin); err == nil {
		return path, nil, nil
	}
	if bin == "ccusage" {
		if path, err := exec.LookPath("bunx"); err == nil {
			return path, []string{"--bun", "ccusage@20.0.20"}, nil
		}
	}
	return "", nil, fmt.Errorf("ccusage is not installed or not on PATH, and bunx is unavailable. Install Bun to let Tokenlens run ccusage automatically, install ccusage 20.0.20, use --ccusage /path/to/ccusage, or try --demo")
}
