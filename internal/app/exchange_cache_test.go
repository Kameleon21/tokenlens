package app

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

func cachedEuro(now time.Time) Exchange {
	return Exchange{Currency: "EUR", Rate: 0.86, Date: "2026-09-04", Source: "ECB via Frankfurter", FetchedAt: now}
}

func TestExchangeCacheStartupAndRefresh(t *testing.T) {
	now := time.Now()
	o := Options{Currency: "EUR", TZ: "UTC", Group: "daily", CacheDir: t.TempDir()}
	x := cachedEuro(now.Add(-23 * time.Hour))
	if err := writeExchangeCache(o, x); err != nil {
		t.Fatal(err)
	}
	m := newModel(context.Background(), o)
	if m.fx.Rate != x.Rate || m.fx.Currency != "EUR" || !m.fx.FetchedAt.Equal(x.FetchedAt) {
		t.Fatalf("cached currency not ready before first frame: %+v", m.fx)
	}
	if strings.Contains(ansi.Strip(m.View()), "ESTIMATED COST · USD") {
		t.Fatal("startup flashed USD")
	}
	if cmd := m.refreshExchangeAt(now); cmd != nil || m.fxLoading {
		t.Fatal("fresh startup fetched a rate")
	}
	// The actual r key must still force usage refresh, without refreshing FX.
	before := m.request
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	m = next.(model)
	if cmd == nil || m.request != before+1 || !m.loading || m.fxLoading || m.fx.Rate != x.Rate {
		t.Fatal("manual refresh did not preserve fresh FX and refresh usage")
	}
	if m.cancel != nil {
		m.cancel()
	}
	if cmd := m.refreshExchangeAt(x.FetchedAt.Add(24*time.Hour - time.Nanosecond)); cmd != nil {
		t.Fatal("rate expired before 24 hours")
	}
	if cmd := m.refreshExchangeAt(x.FetchedAt.Add(24 * time.Hour)); cmd == nil || !m.fxLoading || m.fx.Rate != x.Rate {
		t.Fatal("24-hour-old rate not refreshed while keeping old value")
	}
	defer m.fxCancel()
	id := m.fxRequest
	if cmd := m.refreshExchangeAt(now.Add(2 * time.Hour)); cmd != nil || m.fxRequest != id {
		t.Fatal("duplicate refresh restarted in-flight FX request")
	}
	next, _ = m.Update(exchangeMsg{err: errors.New("offline"), id: id})
	m = next.(model)
	if m.fx.Rate != x.Rate || !m.fx.FetchedAt.Equal(x.FetchedAt) || !strings.Contains(m.exchangeStatus(), "previous rate") {
		t.Fatal("failed refresh lost previous rate or changed its age")
	}
}

func TestExchangeCachePersistsOnlySuccess(t *testing.T) {
	o := Options{Currency: "EUR", CacheDir: t.TempDir()}
	var fail atomic.Bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if fail.Load() {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write([]byte(`{"date":"2026-09-04","base":"USD","quote":"EUR","rate":0.86}`))
	}))
	defer srv.Close()
	start := time.Now()
	x, err := fetchAndCacheExchange(context.Background(), srv.Client(), srv.URL+"/", o)
	if err != nil || x.FetchedAt.Before(start) {
		t.Fatalf("successful fetch missing receipt timestamp: %+v %v", x, err)
	}
	cached, err := readExchangeCache(o, "EUR", time.Now())
	if err != nil || !cached.FetchedAt.Equal(x.FetchedAt) || cached.Rate != x.Rate {
		t.Fatalf("successful rate not persisted: %+v %v", cached, err)
	}
	fail.Store(true)
	if _, err = fetchAndCacheExchange(context.Background(), srv.Client(), srv.URL+"/", o); err == nil {
		t.Fatal("failed fetch succeeded")
	}
	cached, err = readExchangeCache(o, "EUR", time.Now())
	if err != nil || !cached.FetchedAt.Equal(x.FetchedAt) {
		t.Fatal("failed fetch replaced successful cache")
	}
}

func TestExchangeCacheValidationAndIsolation(t *testing.T) {
	now := time.Now()
	o := Options{Currency: "EUR", CacheDir: t.TempDir()}
	valid := cachedEuro(now.Add(-48 * time.Hour))
	if err := writeExchangeCache(o, valid); err != nil {
		t.Fatal(err)
	}
	if x, err := readExchangeCache(o, "EUR", now); err != nil || x.fresh(now) {
		t.Fatal("expired rate must remain readable for fallback")
	}
	if _, err := readExchangeCache(o, "GBP", now); err == nil {
		t.Fatal("different currency shared a rate")
	}
	path, _ := exchangeCachePath(o, "EUR")
	for name, mutate := range map[string]func(*Exchange){
		"zero":               func(x *Exchange) { x.Rate = 0 },
		"negative":           func(x *Exchange) { x.Rate = -1 },
		"currency":           func(x *Exchange) { x.Currency = "GBP" },
		"missing fetch time": func(x *Exchange) { x.FetchedAt = time.Time{} },
		"future fetch time":  func(x *Exchange) { x.FetchedAt = now.Add(time.Hour) },
		"bad date":           func(x *Exchange) { x.Date = "2026-02-30" },
		"source":             func(x *Exchange) { x.Source = "synthetic demo rate" },
	} {
		t.Run(name, func(t *testing.T) {
			x := valid
			mutate(&x)
			data, _ := json.Marshal(x)
			if err := os.WriteFile(path, data, 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := readExchangeCache(o, "EUR", now); err == nil {
				t.Fatal("invalid cache accepted")
			}
		})
	}
	if err := os.WriteFile(path, []byte("broken json"), 0600); err != nil {
		t.Fatal(err)
	}
	if x := initialExchange(o, now); x.Currency != "EUR" || x.available() {
		t.Fatal("corrupt cache must preserve requested currency without a rate")
	}
	for _, currency := range []string{"../EUR", "eur", "EUR/USD"} {
		if _, err := exchangeCachePath(o, currency); err == nil {
			t.Fatal("unsafe currency path accepted")
		}
	}
	for _, mode := range []string{"no-cache", "demo"} {
		t.Run(mode, func(t *testing.T) {
			opts := Options{CacheDir: t.TempDir(), NoCache: mode == "no-cache", Demo: mode == "demo"}
			if err := writeExchangeCache(opts, valid); err != nil {
				t.Fatal(err)
			}
			entries, _ := os.ReadDir(opts.CacheDir)
			if len(entries) != 0 {
				t.Fatal("disabled cache wrote data")
			}
			// Even an existing valid cache must be ignored in these modes.
			if err := writeExchangeCache(Options{CacheDir: opts.CacheDir}, valid); err != nil {
				t.Fatal(err)
			}
			if _, err := readExchangeCache(opts, "EUR", now); err == nil {
				t.Fatal("disabled cache read data")
			}
		})
	}
}

func TestMissingExchangeNeverShowsUSDFallbackOrZeroCost(t *testing.T) {
	m := newModel(context.Background(), Options{Currency: "EUR", TZ: "UTC", Group: "daily", CacheDir: t.TempDir(), PlanCost: 100, PlanCurrency: "EUR"})
	m.s = fixtureModel().s
	for _, size := range [][2]int{{50, 16}, {80, 24}, {120, 40}} {
		m.width, m.height = size[0], size[1]
		for tab := range views {
			m.view = tab
			view := ansi.Strip(m.View())
			if strings.Contains(view, "$") || strings.Contains(view, "ESTIMATED COST · USD") || strings.Contains(view, "€0.0000") {
				t.Fatalf("incorrect amount before exchange loads at %v tab %d:\n%s", size, tab, view)
			}
		}
	}
	if m.fx.Currency != "EUR" || !strings.Contains(m.exchangeStatus(), "loading EUR") || m.fx.format(known(100)) != "unavailable" {
		t.Fatal("missing rate lacks loading state in configured currency")
	}
	if !strings.Contains(m.planSummary(), "pending") {
		t.Fatal("plan comparison used missing exchange rate")
	}
	for _, kind := range []string{"json", "csv", "svg", "png"} {
		if _, err := m.writeExport(kind); err == nil {
			t.Fatal("export used missing exchange rate", kind)
		}
	}
}

func TestExchangeSwitchReusesRatesAndIgnoresStaleResult(t *testing.T) {
	now := time.Now()
	o := Options{Currency: "EUR", CacheDir: t.TempDir()}
	if err := writeExchangeCache(o, cachedEuro(now.Add(-time.Hour))); err != nil {
		t.Fatal(err)
	}
	m := newModel(context.Background(), o)
	m.o.Currency = "GBP"
	if cmd := m.refreshExchangeAt(now); cmd == nil || m.fx.Currency != "GBP" || m.fx.available() {
		t.Fatal("currency switch displayed wrong rate")
	}
	id := m.fxRequest
	m.o.Currency = "EUR"
	if cmd := m.refreshExchangeAt(now); cmd != nil || m.fx.Rate != 0.86 || m.fxLoading {
		t.Fatal("switching back did not reuse fresh euro rate")
	}
	next, _ := m.Update(exchangeMsg{exchange: Exchange{Currency: "GBP", Rate: 0.75, FetchedAt: now}, id: id})
	m = next.(model)
	if m.fx.Currency != "EUR" {
		t.Fatal("stale currency request applied")
	}
	// Disk persistence is optional; the session still remembers a successful rate.
	m = newModel(context.Background(), o)
	m.o.NoCache = true
	m.o.Currency = "USD"
	_ = m.refreshExchangeAt(now)
	m.o.Currency = "EUR"
	if cmd := m.refreshExchangeAt(now); cmd != nil || m.fx.Rate != 0.86 {
		t.Fatal("session lost euro rate after switching through USD")
	}
}

func TestExchangeCacheWriteFailureStillConverts(t *testing.T) {
	path := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(path, nil, 0600); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"date":"2026-09-04","base":"USD","quote":"EUR","rate":0.86}`))
	}))
	defer srv.Close()
	x, err := fetchAndCacheExchange(context.Background(), srv.Client(), srv.URL+"/", Options{Currency: "EUR", CacheDir: path})
	if err != nil || x.format(known(100)) != "€86.0000" {
		t.Fatal("cache write failure prevented valid conversion")
	}
}
