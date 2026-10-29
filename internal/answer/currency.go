package answer

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Rates are the European Central Bank reference rates from Frankfurter
// (frankfurter.dev, no key, open source): one per working day. The daemon
// fetches them into a cache file; a query only ever reads that file.
const ratesURL = "https://api.frankfurter.dev/v1/latest?base=EUR"

type rateTable struct {
	Date  string             `json:"date"`
	Rates map[string]float64 `json:"rates"` // per euro, keys uppercase
}

var rates struct {
	sync.Mutex
	stamp time.Time
	table rateTable
}

func ratesFile() string {
	dir := os.Getenv("XDG_CACHE_HOME")
	if dir == "" {
		dir = filepath.Join(os.Getenv("HOME"), ".cache")
	}
	return filepath.Join(dir, "nzlauncher", "rates.json")
}

// currentRates reads the cache, again only when the file changed.
func currentRates() (rateTable, bool) {
	rates.Lock()
	defer rates.Unlock()
	info, err := os.Stat(ratesFile())
	if err != nil {
		return rateTable{}, false
	}
	if !info.ModTime().Equal(rates.stamp) {
		var t rateTable
		data, err := os.ReadFile(ratesFile())
		if err != nil || json.Unmarshal(data, &t) != nil || len(t.Rates) == 0 {
			return rateTable{}, false
		}
		t.Rates["EUR"] = 1
		rates.stamp, rates.table = info.ModTime(), t
	}
	return rates.table, true
}

// RefreshRates fetches today's rates when the cache is older than a day.
// The daemon calls it in the background; a failure keeps the old file.
func RefreshRates() error {
	if info, err := os.Stat(ratesFile()); err == nil && time.Since(info.ModTime()) < 24*time.Hour {
		return nil
	}
	client := http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(ratesURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("rates: %s", resp.Status)
	}
	var t rateTable
	if err := json.NewDecoder(resp.Body).Decode(&t); err != nil || len(t.Rates) == 0 {
		return fmt.Errorf("rates: unreadable answer")
	}
	data, _ := json.Marshal(t)
	if err := os.MkdirAll(filepath.Dir(ratesFile()), 0o700); err != nil {
		return err
	}
	tmp := ratesFile() + ".part"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, ratesFile())
}

var currencyNames = map[string]string{
	"dollar": "USD", "dollars": "USD", "bucks": "USD", "usd": "USD",
	"euro": "EUR", "euros": "EUR",
	"pound": "GBP", "pounds": "GBP", "quid": "GBP",
	"yen": "JPY", "yuan": "CNY", "renminbi": "CNY", "rmb": "CNY",
	"rupee": "INR", "rupees": "INR", "rs": "INR",
	"won": "KRW", "franc": "CHF", "francs": "CHF", "real": "BRL", "reais": "BRL",
	"lira": "TRY", "zloty": "PLN", "rand": "ZAR", "peso": "MXN", "pesos": "MXN",
	"krona": "SEK", "kronor": "SEK", "krone": "NOK",
}

func currencyCode(w string, t rateTable) (string, bool) {
	if code, ok := currencyNames[w]; ok {
		w = code
	}
	code := strings.ToUpper(w)
	_, ok := t.Rates[code]
	return code, ok
}

// convertCurrency answers "100 usd to inr" from the cached rates.
func convertCurrency(n float64, from, to string, now time.Time) (Answer, bool) {
	t, ok := currentRates()
	if !ok {
		return Answer{}, false
	}
	a, okA := currencyCode(from, t)
	b, okB := currencyCode(to, t)
	if !okA || !okB || a == b {
		return Answer{}, false
	}
	v := n / t.Rates[a] * t.Rates[b]
	value := money(v)
	hint := fmt.Sprintf("%s %s · ECB rate of %s", Format(n), a, t.Date)
	if day, err := time.Parse("2006-01-02", t.Date); err == nil && now.Sub(day) > 4*24*time.Hour {
		hint += " (offline, old rate)"
	}
	return Answer{Value: value, Name: value + " " + b, Hint: hint}, true
}

// money keeps two decimals, more only for amounts under one.
func money(v float64) string {
	if v != 0 && v < 1 && v > -1 {
		return Format(v)
	}
	return strings.TrimSuffix(fmt.Sprintf("%.2f", v), ".00")
}
