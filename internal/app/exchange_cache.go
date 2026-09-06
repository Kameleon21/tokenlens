package app

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

const exchangeCacheTTL = 24 * time.Hour

func (x Exchange) available() bool {
	return x.Rate > 0 && !math.IsNaN(x.Rate) && !math.IsInf(x.Rate, 0)
}

func (x Exchange) fresh(now time.Time) bool {
	age := now.Sub(x.FetchedAt)
	return x.available() && !x.FetchedAt.IsZero() && age >= 0 && age < exchangeCacheTTL
}

func exchangeCachePath(o Options, currency string) (string, error) {
	code, err := currencyCode(currency)
	if err != nil || code != currency {
		return "", fmt.Errorf("invalid exchange-cache currency")
	}
	dir := o.CacheDir
	if dir == "" {
		root, err := os.UserCacheDir()
		if err != nil {
			return "", err
		}
		dir = filepath.Join(root, "tokenlens")
	}
	return filepath.Join(dir, "exchange-v1-USD-"+currency+".json"), nil
}

func readExchangeCache(o Options, currency string, now time.Time) (Exchange, error) {
	if o.NoCache || o.Demo {
		return Exchange{}, os.ErrNotExist
	}
	path, err := exchangeCachePath(o, currency)
	if err != nil {
		return Exchange{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return Exchange{}, err
	}
	var x Exchange
	if err = json.Unmarshal(data, &x); err != nil {
		return Exchange{}, err
	}
	if x.Currency != currency || !x.available() || x.FetchedAt.IsZero() || x.FetchedAt.After(now) || x.Source != "ECB via Frankfurter" {
		return Exchange{}, fmt.Errorf("invalid cached exchange rate")
	}
	if _, err = time.Parse("2006-01-02", x.Date); err != nil {
		return Exchange{}, fmt.Errorf("invalid cached exchange date")
	}
	// Keep expired successful rates available while refreshing or when offline.
	return x, nil
}

func writeExchangeCache(o Options, x Exchange) error {
	if o.NoCache || o.Demo {
		return nil
	}
	path, err := exchangeCachePath(o, x.Currency)
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	data, err := json.Marshal(x)
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".exchange-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err = f.Write(data); err != nil {
		_ = f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}

func fetchAndCacheExchange(ctx context.Context, client *http.Client, endpoint string, o Options) (Exchange, error) {
	x, err := fetchExchange(ctx, client, endpoint, o.Currency)
	if err == nil {
		// An unwritable cache must not prevent conversion for this session.
		_ = writeExchangeCache(o, x)
	}
	return x, err
}

func initialExchange(o Options, now time.Time) Exchange {
	if o.Currency == "USD" {
		return usdExchange()
	}
	if x, err := readExchangeCache(o, o.Currency, now); err == nil {
		return x
	}
	return Exchange{Currency: o.Currency}
}
