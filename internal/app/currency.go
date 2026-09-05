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
)

const exchangeEndpoint = "https://api.frankfurter.dev/v2/rate/USD/"

// Exchange changes presentation only. All usage calculations remain in USD.
type Exchange struct {
	Currency     string
	Rate         float64
	Date, Source string
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
	return Exchange{Currency: currency, Rate: data.Rate, Date: data.Date, Source: "ECB via Frankfurter"}, nil
}
func (x Exchange) format(v Metric) string {
	if !v.Known {
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
