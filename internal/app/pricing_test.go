package app

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func testPrices() priceCatalog {
	return priceCatalog{Version: 1, Fetched: time.Now().UTC(), Source: pricingURL, Models: map[string]map[string]float64{"known": {"inputCostPerToken": 0.000002, "outputCostPerToken": 0.000008, "cacheReadInputTokenCost": 0.0000002}}}
}
func TestPriceCoveragePreservesUnknownAndFreeCosts(t *testing.T) {
	p := testPrices()
	p.Models["free"] = map[string]float64{"inputCostPerToken": 0, "outputCostPerToken": 0}
	row := func(name string, cost float64) Row {
		return Row{Name: name, Usage: Usage{Input: Metric{Value: 10, Known: true}, Tokens: Metric{Value: 10, Known: true}, Cost: Metric{Value: cost, Known: true}}}
	}
	known, unknown, free := row("known", 2), row("new-model", 0), row("free", 0)
	s := Snapshot{Sections: map[string][]Row{"daily": {{Name: "20260901", Usage: Usage{Cost: Metric{Value: 2, Known: true}}, Models: []Row{known, unknown, free}}}}}
	p.applyCoverage(&s)
	r := s.Sections["daily"][0]
	if !r.Usage.Cost.Known || !r.Usage.Cost.Partial || r.Usage.Cost.Value != 2 {
		t.Fatalf("bad partial aggregate: %+v", r.Usage.Cost)
	}
	if r.Models[1].Usage.Cost.Known || !r.Models[2].Usage.Cost.Known {
		t.Fatal("unknown/free prices conflated")
	}
	if len(s.Unpriced) != 1 || s.Unpriced[0] != "new-model" {
		t.Fatalf("bad coverage: %v", s.Unpriced)
	}
	s = Snapshot{Sections: map[string][]Row{"daily": {{Name: "day", Models: []Row{unknown}}}}}
	p.applyCoverage(&s)
	if s.Sections["daily"][0].Usage.Cost.Known {
		t.Fatal("all-unknown cost became known zero")
	}
	known.Usage.Write = Metric{Value: 4, Known: true}
	s = Snapshot{Sections: map[string][]Row{"daily": {{Name: "day", Models: []Row{known}}}}}
	p.applyCoverage(&s)
	if s.Sections["daily"][0].Usage.Cost.Known {
		t.Fatal("missing cache-write price not recognized")
	}
}
func TestPriceFetchValidationAndCancellation(t *testing.T) {
	catalog := `{"known":{"input_cost_per_token":0.000002,"output_cost_per_token":0.000008,"max_input_tokens":200000,"cache_read_input_token_cost":0.0000002,"provider_specific_entry":{"fast":2},"input_cost_per_token_above_200k_tokens":0.000004},"invalid":{"input_cost_per_token":-1,"output_cost_per_token":1},"missing":{"input_cost_per_token":1},"null":{"input_cost_per_token":null,"output_cost_per_token":1}}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(catalog)) }))
	defer server.Close()
	p, err := fetchPrices(context.Background(), server.Client(), server.URL)
	if err != nil || len(p.Models) != 1 {
		t.Fatalf("catalog validation: %v %v", p.Models, err)
	}
	if p.Models["known"]["fastMultiplier"] != 2 || p.Models["known"]["inputCostPerTokenAbove200kTokens"] != 0.000004 {
		t.Fatal("tier pricing lost")
	}
	for _, bad := range []string{`{}`, `{"broken":true}`, `not json`} {
		catalog = bad
		if _, err := fetchPrices(context.Background(), server.Client(), server.URL); err == nil {
			t.Fatal("accepted invalid catalog")
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := fetchPrices(ctx, server.Client(), server.URL); err == nil {
		t.Fatal("ignored cancellation")
	}
}
func TestPriceCacheFallbackAndNoCache(t *testing.T) {
	o := Options{CacheDir: t.TempDir()}
	p := testPrices()
	p.Fetched = time.Now().UTC()
	if err := savePrices(o, p); err != nil {
		t.Fatal(err)
	}
	if got := initialPrices(o); got.revision() != p.revision() {
		t.Fatal("valid downloaded catalog not preferred")
	}
	path, _ := priceCachePath(o)
	os.WriteFile(path, []byte(`broken`), 0600)
	if got := initialPrices(o); len(got.Models) < 10 {
		t.Fatal("corrupt cache did not use bundled catalog")
	}
	o.NoCache = true
	if err := savePrices(o, p); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	if string(data) != "broken" {
		t.Fatal("no-cache wrote prices")
	}
	p.Fetched = time.Now().Add(time.Hour)
	data, _ = json.Marshal(p)
	if _, err := decodeCatalog(data); err == nil {
		t.Fatal("accepted future catalog")
	}
}
func TestPriceUpdatesInvalidateReportsWithoutRestartingInflightLoad(t *testing.T) {
	p := testPrices()
	m := newModel(context.Background(), Options{Demo: true})
	m.o.Demo = false
	m.o.managedPrices = true
	m.prices = p
	m.o.priceRevision = p.revision()
	m.loading = true
	m.request = 7
	next := testPrices()
	next.Models["known"]["inputCostPerToken"] = 3
	value, cmd := m.Update(pricesMsg{catalog: next})
	m = value.(model)
	if cmd != nil || !m.loading || m.request != 7 {
		t.Fatal("price update interrupted active usage load")
	}
	if m.o.priceRevision != next.revision() {
		t.Fatal("new prices not installed")
	}
	// When the old request completes, one new report is requested with the new catalog.
	m.o.Offline = true
	value, cmd = m.Update(loadedMsg{s: Snapshot{PriceRevision: p.revision()}, id: 7})
	m = value.(model)
	if cmd == nil || !m.loading || m.request != 8 {
		t.Fatal("stale-price completion was not repriced")
	}
}
func TestPriceRefreshIsOptOutAndBacksOff(t *testing.T) {
	m := newModel(context.Background(), Options{Demo: true})
	m.o.Demo = false
	m.o.managedPrices = true
	m.prices = testPrices()
	if m.refreshPrices(false) != nil {
		t.Fatal("fresh catalog fetched again")
	}
	m.o.Offline = true
	if m.refreshPrices(true) != nil {
		t.Fatal("offline mode starts network")
	}
	m.o.Offline = false
	m.priceAttempt = time.Now()
	if m.refreshPrices(true) != nil {
		t.Fatal("missing retry backoff")
	}
	m.priceAttempt = time.Time{}
	m.priceLoading = true
	if m.refreshPrices(true) != nil {
		t.Fatal("parallel refresh started")
	}
}
func TestBundledBackendResolvesThroughSymlink(t *testing.T) {
	root := t.TempDir()
	bundle := filepath.Join(root, "release")
	os.MkdirAll(filepath.Join(bundle, "libexec"), 0700)
	name := "ccusage"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	binary := filepath.Join(bundle, "tokenlens")
	os.WriteFile(binary, []byte("test"), 0700)
	backend := filepath.Join(bundle, "libexec", name)
	os.WriteFile(backend, []byte("test"), 0700)
	expected, _ := filepath.EvalSymlinks(backend)
	if got := bundledBackend(binary); got != expected {
		t.Fatalf("companion not found: %q", got)
	}
	if runtime.GOOS != "windows" {
		link := filepath.Join(root, "tokenlens")
		if err := os.Symlink(binary, link); err != nil {
			t.Fatal(err)
		}
		if bundledBackend(link) != expected {
			t.Fatal("symlink installation lost companion")
		}
	}
}
func TestExistingCCUsageConfigurationKeepsItsBehavior(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	t.Setenv("CLAUDE_CONFIG_DIR", filepath.Join(dir, "claude"))
	if hasCCUsageConfig() {
		t.Fatal("unexpected config")
	}
	os.MkdirAll(filepath.Join(dir, ".ccusage"), 0700)
	os.WriteFile(filepath.Join(dir, ".ccusage", "ccusage.json"), []byte(`{"defaults":{"pricingOverrides":{}}}`), 0600)
	if !hasCCUsageConfig() {
		t.Fatal("project config was ignored")
	}
}
func TestLocalPricingConfigPreservesRates(t *testing.T) {
	p := testPrices()
	p.Models["known"]["fastMultiplier"] = 2
	p.Models["known"]["inputCostPerTokenAbove200kTokens"] = 0.000004
	path, cleanup, err := p.configFile()
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var config struct {
		Defaults struct {
			PricingOverrides map[string]map[string]float64 `json:"pricingOverrides"`
		} `json:"defaults"`
	}
	if json.Unmarshal(data, &config) != nil || config.Defaults.PricingOverrides["known"]["fastMultiplier"] != 2 {
		t.Fatal("override missing")
	}
	cleanup()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("temporary config not removed")
	}
	if !strings.Contains(string(data), "inputCostPerTokenAbove200kTokens") {
		t.Fatal("long context rate missing")
	}
}

func TestPriceChangeDuringCachedReuseTriggersRepricing(t *testing.T) {
	old := testPrices()
	m := newModel(context.Background(), Options{Demo: true})
	m.o.Demo = false
	m.o.managedPrices = true
	m.o.Offline = true
	m.prices = old
	m.o.priceRevision = old.revision()
	m.loading = true
	m.request = 4
	updated := testPrices()
	updated.Models["known"]["outputCostPerToken"] = 9
	next, _ := m.Update(pricesMsg{catalog: updated})
	m = next.(model)
	next, cmd := m.Update(reusedMsg{s: Snapshot{PriceRevision: old.revision()}, id: 4})
	m = next.(model)
	if cmd == nil || !m.loading || m.request != 5 {
		t.Fatal("old cached prices were silently reused")
	}
}
