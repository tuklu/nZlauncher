// Package answer computes inline answers: unit and currency conversions,
// the time in other places, and date math. Everything here is offline; the
// currency rates come from a cache the daemon refreshes once a day.
package answer

import (
	"math"
	"strconv"
	"strings"
	"time"
)

// Answer is one computed result. Value is what gets copied.
type Answer struct {
	Kind  string // "units", "currency", "time", "dates"
	Value string
	Name  string
	Hint  string
}

// For answers q, or returns nothing. enabled says which kinds are on.
func For(q string, now time.Time, enabled func(kind string) bool) []Answer {
	q = strings.TrimSpace(strings.ToLower(q))
	if q == "" {
		return nil
	}
	var out []Answer
	add := func(a Answer, ok bool, kind string) {
		if ok && enabled(kind) {
			a.Kind = kind
			out = append(out, a)
		}
	}
	if n, from, to, ok := conversion(q); ok {
		if a, ok := convertUnits(n, from, to); ok {
			add(a, true, "units")
		} else {
			a, ok := convertCurrency(n, from, to, now)
			add(a, ok, "currency")
		}
	}
	if len(out) == 0 {
		a, ok := timeIn(q, now)
		add(a, ok, "time")
	}
	if len(out) == 0 {
		a, ok := dateMath(q, now)
		add(a, ok, "dates")
	}
	return out
}

var conversionWords = map[string]bool{"to": true, "in": true, "into": true, "as": true, "=": true, "->": true}

// conversion reads "5 km to miles", "5km in mi", "$100 to inr", "100 usd inr",
// "convert 5 kg to lb", "how many cm in 5 ft".
func conversion(q string) (n float64, from, to string, ok bool) {
	words := strings.Fields(q)
	for len(words) > 0 && (words[0] == "convert" || words[0] == "what" || words[0] == "is" || words[0] == "whats") {
		words = words[1:]
	}
	// "how many cm in 5 ft"
	if len(words) >= 5 && words[0] == "how" && words[1] == "many" {
		target := words[2]
		if n, unit, used := amount(words[4:]); used > 0 && words[3] == "in" && 4+used == len(words) {
			return n, unit, target, true
		}
		return 0, "", "", false
	}
	n, from, used := amount(words)
	if used == 0 || from == "" {
		return 0, "", "", false
	}
	rest := words[used:]
	if len(rest) == 2 && conversionWords[rest[0]] {
		return n, from, rest[1], true
	}
	// "100 usd inr": only for currencies, units need the word
	if len(rest) == 1 && len(rest[0]) == 3 && len(from) == 3 {
		return n, from, rest[0], true
	}
	return 0, "", "", false
}

var currencySigns = map[string]string{"$": "usd", "€": "eur", "£": "gbp", "¥": "jpy", "₹": "inr", "₩": "krw", "₽": "rub"}

// amount reads a number and its unit from the front of words: "5 km",
// "5km", "$100", "100usd", "72°f". used is how many words it took.
func amount(words []string) (n float64, unit string, used int) {
	if len(words) == 0 {
		return 0, "", 0
	}
	w := words[0]
	for sign, code := range currencySigns {
		if rest, ok := strings.CutPrefix(w, sign); ok {
			if v, err := parseNumber(rest); err == nil {
				return v, code, 1
			}
		}
	}
	k := 0
	for k < len(w) && (w[k] >= '0' && w[k] <= '9' || w[k] == '.' || w[k] == ',' || k == 0 && w[k] == '-') {
		k++
	}
	v, err := parseNumber(w[:k])
	if err != nil {
		return 0, "", 0
	}
	if k < len(w) {
		return v, w[k:], 1
	}
	if len(words) > 1 {
		u := words[1]
		// "5 fl oz"
		if u == "fl" && len(words) > 2 && words[2] == "oz" {
			return v, "floz", 3
		}
		return v, u, 2
	}
	return v, "", 1
}

func parseNumber(s string) (float64, error) {
	return strconv.ParseFloat(strings.ReplaceAll(s, ",", ""), 64)
}

// Format prints a number the way a person would write it: up to six
// significant digits, no exponent in everyday ranges, no trailing zeros.
func Format(v float64) string {
	if v == 0 {
		return "0"
	}
	abs := math.Abs(v)
	if abs >= 1e15 || abs < 1e-6 {
		return strconv.FormatFloat(v, 'g', 6, 64)
	}
	digits := 6 - int(math.Floor(math.Log10(abs))) - 1
	if digits < 0 {
		digits = 0
	}
	if abs >= 1000 {
		digits = min(digits, 2)
	}
	s := strconv.FormatFloat(v, 'f', digits, 64)
	if strings.Contains(s, ".") {
		s = strings.TrimRight(strings.TrimRight(s, "0"), ".")
	}
	return s
}
