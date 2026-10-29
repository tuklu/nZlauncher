package answer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func all(string) bool { return true }

// Thursday 29 October 2026, 15:00 in India
var now = time.Date(2026, 10, 29, 15, 0, 0, 0, time.FixedZone("IST", 5*3600+1800))

func answer(t *testing.T, q string) Answer {
	t.Helper()
	got := For(q, now, all)
	if len(got) == 0 {
		t.Fatalf("%q: no answer", q)
	}
	return got[0]
}

func TestUnits(t *testing.T) {
	cases := map[string]string{
		"5 km to miles":         "3.10686 mi",
		"5km in mi":             "3.10686 mi",
		"72f to c":              "22.2222 °C",
		"100 c in f":            "212 °F",
		"-40 c to f":            "-40 °F",
		"1.5 gb to mib":         "1430.51 MiB",
		"6 ft in cm":            "182.88 cm",
		"how many cm in 6 ft":   "182.88 cm",
		"convert 10 lb to kg":   "4.53592 kg",
		"2 cups to ml":          "473.176 ml",
		"100 kmh to mph":        "62.1371 mph",
		"1 acre in m2":          "4046.86 m²",
		"90 minutes in hours":   "1.5 h",
		"1,000,000 bytes to mb": "1 MB",
	}
	for q, want := range cases {
		if got := answer(t, q).Name; got != want {
			t.Errorf("%q: got %q, want %q", q, got, want)
		}
	}
	for _, q := range []string{"5 km", "firefox", "kg to lb", "5 km to kg"} {
		if got := For(q, now, all); len(got) != 0 {
			t.Errorf("%q: want nothing, got %+v", q, got)
		}
	}
}

func TestCurrency(t *testing.T) {
	cache := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cache)
	os.MkdirAll(filepath.Join(cache, "nzlauncher"), 0o700)
	os.WriteFile(filepath.Join(cache, "nzlauncher", "rates.json"),
		[]byte(`{"date":"2026-10-29","rates":{"USD":1.113,"INR":106.04,"GBP":0.8358}}`), 0o600)

	cases := map[string]string{
		"100 usd to inr":   "9527.40 INR",
		"$100 in inr":      "9527.40 INR",
		"100 usd inr":      "9527.40 INR",
		"50 euros to usd":  "55.65 USD",
		"10 pounds to usd": "13.32 USD", // a currency, because usd is not a weight
		"10 pounds to kg":  "4.53592 kg",
		"1 inr to usd":     "0.010496 USD",
	}
	for q, want := range cases {
		if got := answer(t, q).Name; got != want {
			t.Errorf("%q: got %q, want %q", q, got, want)
		}
	}
	if hint := answer(t, "100 usd to inr").Hint; !strings.Contains(hint, "2026-10-29") {
		t.Errorf("hint should date the rate: %q", hint)
	}
	if got := For("100 usd to inr", now, func(k string) bool { return k != "currency" }); len(got) != 0 {
		t.Errorf("currency off still answered: %+v", got)
	}
}

func TestTimeIn(t *testing.T) {
	if _, err := os.Stat(zoneinfo + "/zone1970.tab"); err != nil {
		t.Skip("no tz database")
	}
	cases := map[string]string{
		"time in tokyo":                "18:30 in Tokyo",
		"tokyo time":                   "18:30 in Tokyo",
		"what time is it in new york?": "05:30 in New York",
		"time in japan":                "18:30 in Japan",
		"time in pst":                  "02:30 in PST",
		"3pm in london":                "09:30 in London",
		"3pm london to tokyo":          "00:00 Fri in Tokyo", // UK is back on GMT from 25 Oct
		"15:00 utc in ist":             "20:30 in IST",
		"9am new york to delhi":        "18:30 in Delhi",
	}
	for q, want := range cases {
		if got := answer(t, q).Name; got != want {
			t.Errorf("%q: got %q, want %q", q, got, want)
		}
	}
	if got := answer(t, "time in tokyo").Hint; got != "Asia/Tokyo · UTC+9 · 3h 30m ahead" {
		t.Errorf("hint: %q", got)
	}
}

func TestDates(t *testing.T) {
	cases := map[string]string{
		"days until christmas":    "57 days",
		"how long until new year": "64 days",
		"days since 2026-01-01":   "301 days",
		"days since christmas":    "308 days",
		"in 30 days":              "Sat 28 Nov 2026",
		"30 days from now":        "Sat 28 Nov 2026",
		"today + 2 weeks":         "Thu 12 Nov 2026",
		"10 days ago":             "Mon 19 Oct 2026",
		"what day is 2026-12-25":  "Friday, 25 December 2026",
		"2026-12-25":              "Friday, 25 December 2026",
		"week number":             "Week 44",
		"1761750000":              "2025-10-29 20:30:00",
	}
	for q, want := range cases {
		if got := answer(t, q).Name; got != want {
			t.Errorf("%q: got %q, want %q", q, got, want)
		}
	}
	for _, q := range []string{"9876543210", "christmas", "days"} {
		if got := For(q, now, all); len(got) != 0 {
			t.Errorf("%q: want nothing, got %+v", q, got)
		}
	}
}
