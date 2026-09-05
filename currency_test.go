package main

import (
	"context"
	"errors"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
)

func TestCurrencyOptions(t *testing.T) {
	t.Setenv("TOKENLENS_CURRENCY", "EUR")
	o, e := options(nil, time.Now())
	if e != nil || o.Currency != "EUR" {
		t.Fatalf("default: %+v %v", o, e)
	}
	o, e = options([]string{"--currency", "gbp"}, time.Now())
	if e != nil || o.Currency != "GBP" {
		t.Fatalf("override: %+v %v", o, e)
	}
	for _, s := range []string{"EU", "EUR/USD", "€", "123", ""} {
		if _, e = options([]string{"--currency", s}, time.Now()); e == nil {
			t.Fatalf("accepted %q", s)
		}
	}
}
func TestExchangeFormatting(t *testing.T) {
	x := Exchange{Currency: "EUR", Rate: 0.86}
	if s := x.format(known(100)); s != "€86.0000" {
		t.Fatal(s)
	}
	if s := x.format(Metric{Value: 100, Known: true, Partial: true}); s != "€86.0000 + ?" {
		t.Fatal(s)
	}
	if s := x.format(Metric{}); s != "unavailable" {
		t.Fatal(s)
	}
	if s := x.format(known(math.MaxFloat64)); strings.Contains(s, "Inf") {
		t.Fatal(s)
	}
	m := fixtureModel()
	m.fx = x
	if m.formatMetric(known(100), false) != "100" {
		t.Fatal("tokens converted")
	}
	before := total(m.s.Sections["daily"]).Cost.Value
	_ = m.View()
	_ = m.View()
	if total(m.s.Sections["daily"]).Cost.Value != before {
		t.Fatal("USD data mutated")
	}
}
func TestFetchExchange(t *testing.T) {
	cases := []struct {
		name, body string
		status     int
		bad        bool
	}{
		{"valid", `{"date":"2026-09-04","base":"USD","quote":"EUR","rate":0.86}`, 200, false},
		{"wrong pair", `{"date":"2026-09-04","base":"EUR","quote":"USD","rate":1.1}`, 200, true},
		{"zero", `{"date":"2026-09-04","base":"USD","quote":"EUR","rate":0}`, 200, true},
		{"negative", `{"date":"2026-09-04","base":"USD","quote":"EUR","rate":-1}`, 200, true},
		{"bad date", `{"date":"2026-02-30","base":"USD","quote":"EUR","rate":0.86}`, 200, true},
		{"missing date", `{"base":"USD","quote":"EUR","rate":0.86}`, 200, true},
		{"invalid json", `not json`, 200, true},
		{"unknown currency", `{}`, 404, true},
		{"server error", `{}`, 503, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/EUR" || r.URL.Query().Get("providers") != "ECB" {
					t.Errorf("wrong URL %s", r.URL)
				}
				w.WriteHeader(c.status)
				_, _ = w.Write([]byte(c.body))
			}))
			defer srv.Close()
			x, e := fetchExchange(context.Background(), srv.Client(), srv.URL+"/", "EUR")
			if (e != nil) != c.bad {
				t.Fatalf("%+v %v", x, e)
			}
			if !c.bad && (x.Rate != 0.86 || x.Date != "2026-09-04" || x.Source != "ECB via Frankfurter") {
				t.Fatal(x)
			}
		})
	}
}
func TestExchangeCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, e := fetchExchange(ctx, http.DefaultClient, "https://example.invalid/", "EUR")
	if e == nil {
		t.Fatal("canceled request succeeded")
	}
}
func TestExchangeFallbackAndStaleResult(t *testing.T) {
	m := fixtureModel()
	m.o.Currency = "EUR"
	m.fxRequest = 2
	m.fxLoading = true
	v, _ := m.Update(exchangeMsg{exchange: Exchange{Currency: "EUR", Rate: 0.86}, id: 1})
	m = v.(model)
	if m.fx.Currency != "USD" || !m.fxLoading {
		t.Fatal("stale rate applied")
	}
	v, _ = m.Update(exchangeMsg{err: errors.New("offline"), id: 2})
	m = v.(model)
	if m.fx.Currency != "USD" || !strings.Contains(m.exchangeStatus(), "showing USD") {
		t.Fatal("dishonest fallback")
	}
	v, _ = m.Update(exchangeMsg{exchange: Exchange{Currency: "EUR", Rate: 0.86, Date: "2026-09-04", Source: "ECB via Frankfurter"}, id: 2})
	m = v.(model)
	if !strings.Contains(m.exchangeStatus(), "2026-09-04") {
		t.Fatal("missing date")
	}
	v, _ = m.Update(exchangeMsg{err: errors.New("offline"), id: 2})
	m = v.(model)
	if m.fx.Currency != "EUR" || !strings.Contains(m.exchangeStatus(), "previous rate") {
		t.Fatal("cached rate not marked")
	}
}
func TestEuroViewsAndLayout(t *testing.T) {
	m := fixtureModel()
	m.o.Currency = "EUR"
	m.fx = Exchange{Currency: "EUR", Rate: 0.86, Date: "2026-09-04", Source: "ECB via Frankfurter"}
	for _, wh := range [][2]int{{50, 16}, {80, 24}, {120, 40}} {
		m.width, m.height = wh[0], wh[1]
		for view := 0; view < 5; view++ {
			m.view = view
			for _, detail := range []bool{false, true} {
				m.details = detail
				s := ansi.Strip(m.View())
				if strings.Contains(s, "$") || strings.Contains(s, "EST. USD") {
					t.Fatal("unconverted cost in euro view", view)
				}
				if len(strings.Split(s, "\n")) > m.height {
					t.Fatal("height overflow", wh)
				}
				for _, line := range strings.Split(s, "\n") {
					if ansi.StringWidth(line) > m.width {
						t.Fatal("width overflow", wh)
					}
				}
			}
		}
	}
	m.width = 120
	m.height = 40
	m.details = false
	m.view = 0
	s := ansi.Strip(m.View())
	if !strings.Contains(s, "ESTIMATED COST · EUR") || !strings.Contains(s, "€") || !strings.Contains(s, "2026-09-04") {
		t.Fatal("missing euro attribution")
	}
}
func TestLiveExchangeOptIn(t *testing.T) {
	if os.Getenv("TOKENLENS_TEST_LIVE_FX") == "" {
		t.Skip("optional network integration")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	x, e := fetchExchange(ctx, http.DefaultClient, exchangeEndpoint, "EUR")
	if e != nil {
		t.Fatal(e)
	}
	t.Log(x.label())
}
