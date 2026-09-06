package app

import (
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

//go:embed pricedata/litellm.json
var embeddedPrices []byte

const pricingURL = "https://raw.githubusercontent.com/BerriAI/litellm/main/model_prices_and_context_window.json"
const priceTTL = 6 * time.Hour
const maxCatalogBytes = 32 << 20

var priceFields = map[string]string{
	"max_input_tokens":     "maxInputTokens",
	"input_cost_per_token": "inputCostPerToken", "output_cost_per_token": "outputCostPerToken",
	"cache_read_input_token_cost": "cacheReadInputTokenCost", "cache_creation_input_token_cost": "cacheCreationInputTokenCost",
	"input_cost_per_token_above_200k_tokens": "inputCostPerTokenAbove200kTokens", "output_cost_per_token_above_200k_tokens": "outputCostPerTokenAbove200kTokens",
	"cache_read_input_token_cost_above_200k_tokens": "cacheReadInputTokenCostAbove200kTokens", "cache_creation_input_token_cost_above_200k_tokens": "cacheCreationInputTokenCostAbove200kTokens",
}

type priceCatalog struct {
	Version int                           `json:"version"`
	Fetched time.Time                     `json:"fetched"`
	Source  string                        `json:"source"`
	Models  map[string]map[string]float64 `json:"models"`
}

func (p priceCatalog) validate() error {
	if p.Version != 1 || p.Source != pricingURL || p.Fetched.IsZero() || p.Fetched.After(time.Now().Add(time.Minute)) || len(p.Models) == 0 {
		return fmt.Errorf("invalid price catalog metadata")
	}
	for name, rates := range p.Models {
		if name == "" {
			return fmt.Errorf("empty model name")
		}
		if _, ok := rates["inputCostPerToken"]; !ok {
			return fmt.Errorf("missing input rate")
		}
		if _, ok := rates["outputCostPerToken"]; !ok {
			return fmt.Errorf("missing output rate")
		}
		for field, value := range rates {
			allowed := field == "fastMultiplier"
			for _, known := range priceFields {
				if field == known {
					allowed = true
				}
			}
			if field == "maxInputTokens" && (value != math.Trunc(value) || value > float64(1<<53)) {
				return fmt.Errorf("invalid context limit")
			}
			if !allowed || value < 0 || math.IsNaN(value) || math.IsInf(value, 0) {
				return fmt.Errorf("invalid model rate")
			}
		}
	}
	return nil
}
func (p priceCatalog) revision() string {
	data, _ := json.Marshal(p.Models)
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
func decodeCatalog(data []byte) (priceCatalog, error) {
	var p priceCatalog
	if len(data) > maxCatalogBytes {
		return p, fmt.Errorf("price catalog too large")
	}
	if err := json.Unmarshal(data, &p); err != nil {
		return p, err
	}
	return p, p.validate()
}
func priceCachePath(o Options) (string, error) {
	dir := o.CacheDir
	if dir == "" {
		root, err := os.UserCacheDir()
		if err != nil {
			return "", err
		}
		dir = filepath.Join(root, "tokenlens")
	}
	return filepath.Join(dir, "prices-v1.json"), nil
}
func initialPrices(o Options) priceCatalog {
	embedded, err := decodeCatalog(embeddedPrices)
	if err != nil {
		panic("invalid embedded pricing: " + err.Error())
	}
	if !o.NoCache {
		path, e := priceCachePath(o)
		if e == nil {
			if data, e := os.ReadFile(path); e == nil {
				if cached, e := decodeCatalog(data); e == nil && cached.Fetched.After(embedded.Fetched) {
					return cached
				}
			}
		}
	}
	return embedded
}
func fetchPrices(ctx context.Context, client *http.Client, url string) (priceCatalog, error) {
	p := priceCatalog{Version: 1, Source: pricingURL, Fetched: time.Now().UTC(), Models: map[string]map[string]float64{}}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return p, err
	}
	response, err := client.Do(req)
	if err != nil {
		return p, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return p, fmt.Errorf("pricing HTTP %d", response.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, maxCatalogBytes+1))
	if err != nil {
		return p, err
	}
	if len(data) > maxCatalogBytes {
		return p, fmt.Errorf("price catalog too large")
	}
	var raw map[string]json.RawMessage
	if err = json.Unmarshal(data, &raw); err != nil {
		return p, err
	}
	for name, data := range raw {
		var entry map[string]json.RawMessage
		if json.Unmarshal(data, &entry) != nil {
			continue
		}
		rates := map[string]float64{}
		valid := true
		for source, target := range priceFields {
			if value, ok := entry[source]; ok {
				var number float64
				if string(value) == "null" || json.Unmarshal(value, &number) != nil || number < 0 {
					valid = false
					break
				}
				rates[target] = number
			}
		}
		var provider map[string]json.RawMessage
		if json.Unmarshal(entry["provider_specific_entry"], &provider) == nil {
			if raw, ok := provider["fast"]; ok {
				var n float64
				if string(raw) == "null" || json.Unmarshal(raw, &n) != nil || n < 0 {
					valid = false
				} else {
					rates["fastMultiplier"] = n
				}
			}
		}
		_, input := rates["inputCostPerToken"]
		_, output := rates["outputCostPerToken"]
		if valid && input && output {
			p.Models[name] = rates
		}
	}
	return p, p.validate()
}
func savePrices(o Options, p priceCatalog) error {
	if o.NoCache {
		return nil
	}
	path, err := priceCachePath(o)
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	data, err := json.Marshal(p)
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".prices-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err = f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}

// ccusage's own config discovery must remain authoritative. Custom configuration
// and explicit backends keep their original pricing behavior instead of being
// silently replaced with a generated config.
func hasCCUsageConfig() bool {
	cwd, _ := os.Getwd()
	paths := []string{filepath.Join(cwd, ".ccusage", "ccusage.json")}
	if dirs, ok := os.LookupEnv("CLAUDE_CONFIG_DIR"); ok {
		for _, dir := range strings.Split(dirs, ",") {
			if dir = strings.TrimSpace(dir); dir != "" {
				paths = append(paths, filepath.Join(dir, "ccusage.json"))
			}
		}
	} else if home, err := os.UserHomeDir(); err == nil {
		paths = append(paths, filepath.Join(home, ".config", "claude", "ccusage.json"), filepath.Join(home, ".claude", "ccusage.json"))
	}
	for _, path := range paths {
		if _, err := os.Stat(path); err == nil {
			return true
		}
	}
	return false
}
func (p priceCatalog) configFile() (string, func(), error) {
	data, err := json.Marshal(map[string]any{"defaults": map[string]any{"pricingOverrides": p.Models}})
	if err != nil {
		return "", func() {}, err
	}
	f, err := os.CreateTemp("", "tokenlens-pricing-*.json")
	if err != nil {
		return "", func() {}, err
	}
	cleanup := func() { os.Remove(f.Name()) }
	if _, err = f.Write(data); err != nil {
		f.Close()
		cleanup()
		return "", func() {}, err
	}
	if err = f.Close(); err != nil {
		cleanup()
		return "", func() {}, err
	}
	return f.Name(), cleanup, nil
}

// The backend omits missing-pricing metadata from JSON. Require exact catalog
// coverage for every token category used; never present an unknown cost as free.
func (p priceCatalog) applyCoverage(s *Snapshot) {
	missing := map[string]bool{}
	var visit func(Row) Row
	visit = func(r Row) Row {
		for i := range r.Agents {
			r.Agents[i] = visit(r.Agents[i])
		}
		if len(r.Models) > 0 {
			cost := Metric{}
			for i := range r.Models {
				r.Models[i] = visit(r.Models[i])
				cost.add(r.Models[i].Usage.Cost)
			}
			r.Usage.Cost = cost
		} else {
			rates, ok := p.Models[r.Name]
			for field, metric := range map[string]Metric{"inputCostPerToken": r.Usage.Input, "outputCostPerToken": r.Usage.Output, "cacheReadInputTokenCost": r.Usage.Read, "cacheCreationInputTokenCost": r.Usage.Write} {
				if metric.Known && metric.Value > 0 {
					if _, found := rates[field]; !found {
						ok = false
					}
				}
			}
			if !ok && (r.Usage.Tokens.Value > 0 || r.Usage.Input.Value+r.Usage.Output.Value+r.Usage.Read.Value+r.Usage.Write.Value > 0) {
				r.Usage.Cost = Metric{}
				missing[r.Name] = true
			}
		}
		return r
	}
	for section, rows := range s.Sections {
		for i := range rows {
			rows[i] = visit(rows[i])
		}
		s.Sections[section] = rows
	}
	s.PriceDate = p.Fetched
	s.PriceRevision = p.revision()
	s.PriceSource = "LiteLLM"
	for name := range missing {
		s.Unpriced = append(s.Unpriced, name)
	}
	sort.Strings(s.Unpriced)
}

type priceTickMsg struct{}

func priceTick() tea.Cmd {
	return tea.Tick(priceTTL, func(time.Time) tea.Msg { return priceTickMsg{} })
}

type pricesMsg struct {
	catalog priceCatalog
	err     error
}

func (m *model) refreshPrices(force bool) tea.Cmd {
	if !m.o.managedPrices || m.o.Demo || m.o.Offline || m.priceLoading {
		return nil
	}
	now := time.Now()
	if (!force && now.Sub(m.prices.Fetched) < priceTTL) || (!m.priceAttempt.IsZero() && now.Sub(m.priceAttempt) < time.Minute) {
		return nil
	}
	m.priceAttempt = now
	m.priceLoading = true
	ctx := m.ctx
	o := m.o
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
		defer cancel()
		p, err := fetchPrices(ctx, &http.Client{Timeout: 8 * time.Second}, pricingURL)
		if err == nil {
			err = savePrices(o, p)
		}
		return pricesMsg{p, err}
	}
}
func (m model) pricingStatus() string {
	if m.o.Demo {
		return "Synthetic demo prices"
	}
	if m.s.PriceDate.IsZero() {
		return "Prices managed by ccusage"
	}
	status := "Prices " + m.formatTimestamp(m.s.PriceDate) + " · LiteLLM"
	if len(m.s.Unpriced) > 0 {
		status = fmt.Sprintf("Partial prices: %d unpriced · ", len(m.s.Unpriced)) + m.formatTimestamp(m.s.PriceDate)
	}
	if m.priceLoading {
		status += " · updating"
	} else if m.priceErr != "" {
		status += " · update failed"
	} else if time.Since(m.s.PriceDate) > priceTTL {
		status += " · stale"
	}
	return status
}
