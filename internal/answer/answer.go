// Package answer computes inline answers: unit and currency conversions,
// the time in other places, and date math. Everything here is offline; the
// currency rates come from a cache the daemon refreshes once a day.
package answer

import (
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/tuklu/nZlauncher/internal/intent"
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
	// "100 usd to inr?", "what's 5 km in miles"
	q = strings.TrimRight(q, "?!. ")
	for _, lead := range []string{"what's ", "whats ", "what’s ", "what is "} {
		if rest, ok := strings.CutPrefix(q, lead); ok {
			q = "what " + rest
		}
	}
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
	for len(words) > 0 && (words[0] == "convert" || words[0] == "what" || words[0] == "is") {
		words = words[1:]
	}
	// "how many cm in 5 ft"
	if len(words) >= 5 && words[0] == "how" && words[1] == "many" {
		target := unitWord(words[2])
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
	// "44 degrees", "44°c": into the other temperature unit
	saysDegrees := used > 1 || strings.ContainsRune(words[0], '°') // not a bare "44c"
	if len(rest) == 0 && units[from].kind == "temp" && saysDegrees {
		to := "c"
		if units[from].name == "°C" {
			to = "f"
		}
		return n, from, to, true
	}
	// "in degrees fahrenheit", "to ° f", "to nautical miles"
	if len(rest) == 3 && conversionWords[rest[0]] && (degreeWords[rest[1]] || rest[1] == "nautical") {
		if rest[1] == "nautical" {
			rest[2] = "nmi"
		}
		rest = []string{rest[0], rest[2]}
	}
	if len(rest) == 2 && conversionWords[rest[0]] {
		return n, from, unitWord(rest[1]), true
	}
	// "100 usd inr": only for currencies, units need the word
	if len(rest) == 1 && len(rest[0]) == 3 && len(from) == 3 {
		return n, from, unitWord(rest[0]), true
	}
	return 0, "", "", false
}

var currencySigns = map[string]string{"$": "usd", "€": "eur", "£": "gbp", "¥": "jpy", "₹": "inr", "₩": "krw", "₽": "rub"}

// amount reads a number and its unit from the front of words: "5 km",
// "5km", "$100", "100$", "$ 100", "100 $", "72°f", "44 degrees celsius".
// used is how many words it took.
func amount(words []string) (n float64, unit string, used int) {
	if len(words) == 0 {
		return 0, "", 0
	}
	w := words[0]
	// a sign on its own before the number: "$ 100"
	if code, ok := currencySigns[w]; ok && len(words) > 1 {
		if v, err := parseNumber(words[1]); err == nil {
			return v, code, 2
		}
	}
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
		// "5k inr", "1.5k usd": thousands before a currency or unit
		if w[k:] == "k" && len(words) > 1 && !conversionWords[words[1]] {
			return v * 1000, unitWord(words[1]), 2
		}
		return v, unitWord(w[k:]), 1
	}
	used = 1
	// "44 degrees celsius", "44 deg f", "44 ° c"
	if len(words) > used && degreeWords[words[used]] {
		used++
		if len(words) == used || conversionWords[words[used]] {
			return v, localTemperature(), used // "44 degrees in fahrenheit": the local unit unless said
		}
	}
	if len(words) > used {
		u := words[used]
		// "5 fl oz", "1 nautical mile"
		if u == "fl" && len(words) > used+1 && words[used+1] == "oz" {
			return v, "floz", used + 2
		}
		if u == "nautical" && len(words) > used+1 && strings.HasPrefix(words[used+1], "mile") {
			return v, "nmi", used + 2
		}
		return v, unitWord(u), used + 1
	}
	return v, "", used
}

var degreeWords = map[string]bool{"degree": true, "degrees": true, "deg": true, "°": true}

// unitWord turns a currency sign into its code and fixes a typo in a unit
// or currency name: "celcius", "farenheit", "kilometre".
func unitWord(w string) string {
	if code, ok := currencySigns[w]; ok {
		return code
	}
	w = strings.TrimPrefix(w, "°")
	if _, ok := units[w]; ok {
		return w
	}
	if _, ok := currencyNames[w]; ok {
		return w
	}
	if fix, ok := unitVocabulary.Correct(w); ok {
		return fix
	}
	return w
}

// unitVocabulary is every unit and currency name a typo can be fixed to.
var unitVocabulary = func() *intent.Vocab {
	v := map[string]bool{}
	for w := range units {
		v[w] = true
	}
	for w := range currencyNames {
		v[w] = true
	}
	return intent.NewVocab(v)
}()

// parseNumber reads "1,000" and "1,000.5" with comma thousands, and "1,5"
// and "1.000,50" with a comma as the decimal mark: a comma is decimal when
// it is the last mark and fewer than three digits follow it.
func parseNumber(s string) (float64, error) {
	if c := strings.LastIndexByte(s, ','); c >= 0 && c > strings.LastIndexByte(s, '.') && len(s)-c-1 != 3 {
		s = strings.ReplaceAll(s[:c], ".", "") + "." + s[c+1:]
	}
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
