package app

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

const exchangeEndpoint = "https://api.frankfurter.dev/v2/rate/USD/"

// Exchange changes presentation only. All usage calculations remain in USD.
type Exchange struct {
	Currency     string
	Rate         float64
	Date, Source string
	FetchedAt    time.Time
}

func usdExchange() Exchange { return Exchange{Currency: "USD", Rate: 1} }
func currencyCode(s string) (string, error) {
	s = strings.ToUpper(strings.TrimSpace(s))
	if len(s) != 3 {
		return "", fmt.Errorf("--currency must be a three-letter currency code, such as EUR or USD")
	}
	for _, r := range s {
		if r < 'A' || r > 'Z' {
			return "", fmt.Errorf("--currency must contain only A–Z")
		}
	}
	return s, nil
}
func fetchExchange(ctx context.Context, client *http.Client, endpoint, currency string) (Exchange, error) {
	req, e := http.NewRequestWithContext(ctx, http.MethodGet, endpoint+currency+"?providers=ECB", nil)
	if e != nil {
		return Exchange{}, e
	}
	req.Header.Set("Accept", "application/json")
	res, e := client.Do(req)
	if e != nil {
		return Exchange{}, fmt.Errorf("exchange rate unavailable: %w", e)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return Exchange{}, fmt.Errorf("exchange rate unavailable for %s (HTTP %d)", currency, res.StatusCode)
	}
	var data struct {
		Base  string  `json:"base"`
		Quote string  `json:"quote"`
		Date  string  `json:"date"`
		Rate  float64 `json:"rate"`
	}
	dec := json.NewDecoder(io.LimitReader(res.Body, 64*1024))
	if e = dec.Decode(&data); e != nil {
		return Exchange{}, fmt.Errorf("invalid exchange-rate response: %w", e)
	}
	if data.Base != "USD" || data.Quote != currency || data.Rate <= 0 || math.IsNaN(data.Rate) || math.IsInf(data.Rate, 0) {
		return Exchange{}, fmt.Errorf("invalid exchange-rate pair or value")
	}
	if _, e = time.Parse("2006-01-02", data.Date); e != nil {
		return Exchange{}, fmt.Errorf("invalid exchange-rate date")
	}
	return Exchange{Currency: currency, Rate: data.Rate, Date: data.Date, Source: "ECB via Frankfurter", FetchedAt: time.Now()}, nil
}
func (x Exchange) format(v Metric) string {
	if !v.Known || !x.available() {
		return "unavailable"
	}
	amount := v.Value * x.Rate
	if math.IsInf(amount, 0) || math.IsNaN(amount) {
		return "unavailable"
	}
	prefix := x.Currency + " "
	switch x.Currency {
	case "USD":
		prefix = "$"
	case "EUR":
		prefix = "€"
	case "GBP":
		prefix = "£"
	case "JPY":
		prefix = "¥"
	}
	s := fmt.Sprintf("%s%.4f", prefix, amount)
	if v.Partial {
		s += " + ?"
	}
	return s
}
func (x Exchange) label() string {
	if x.Currency == "USD" {
		return ""
	}
	return fmt.Sprintf("FX  1 USD = %.6f %s · %s · %s", x.Rate, x.Currency, x.Date, x.Source)
}

type exchangeMsg struct {
	exchange Exchange
	err      error
	id       int
}

type exchangeState struct {
	fx        Exchange
	fxLoading bool
	fxErr     string
	fxCancel  context.CancelFunc
	fxRequest int
	fxTarget  string
	exchanges map[string]Exchange
}

func (x *exchangeState) stopExchange() {
	if x.fxCancel != nil {
		x.fxCancel()
	}
}

func (x *exchangeState) rememberExchange(e Exchange) {
	if x.exchanges == nil {
		x.exchanges = make(map[string]Exchange)
	}
	x.exchanges[e.Currency] = e
}

func (x *exchangeState) receiveExchange(v exchangeMsg) {
	x.fxLoading = false
	if v.err != nil {
		x.fxErr = v.err.Error()
		return
	}
	x.fx = v.exchange
	x.rememberExchange(v.exchange)
	x.fxErr = ""
}

func (m model) handleExchange(v exchangeMsg) (tea.Model, tea.Cmd) {
	if v.id != m.fxRequest {
		return m, nil
	}
	m.receiveExchange(v)
	return m.settle(v)
}

func (m *model) refreshExchange() tea.Cmd {
	return m.refreshExchangeAt(time.Now())
}

func (m *model) refreshExchangeAt(now time.Time) tea.Cmd {
	if m.fxLoading && m.fxTarget == m.o.Currency {
		return nil
	}
	m.stopExchange()
	m.fxRequest++
	m.fxLoading = false
	m.fxTarget = m.o.Currency
	if m.fx.Currency != m.o.Currency {
		if m.fx.available() {
			m.rememberExchange(m.fx)
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
