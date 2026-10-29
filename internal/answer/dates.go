package answer

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/tuklu/nZlauncher/internal/intent"
)

// dateMath answers "days until christmas", "days since 2026-01-01",
// "in 30 days", "30 days from now", "today + 2 weeks", "10 days ago",
// "what day is 2026-12-25", "week number", and a unix timestamp.
func dateMath(q string, now time.Time) (Answer, bool) {
	today := startOfDay(now)
	q = strings.TrimSuffix(strings.TrimPrefix(q, "what "), "?")
	words := strings.Fields(q)
	if len(words) == 0 {
		return Answer{}, false
	}

	// "days until christmas", "how long until new year", "days since 2026-01-01"
	for _, w := range []string{"until", "till", "to", "since", "from"} {
		before, after, ok := strings.Cut(q, " "+w+" ")
		if !ok || !strings.HasSuffix(before, "days") && !strings.HasSuffix(before, "long") && !strings.HasSuffix(before, "weeks") {
			continue
		}
		// "30 days from today" is an offset, below
		if first, _, _ := strings.Cut(before, " "); first != "days" && first != "how" && first != "weeks" {
			if _, isNumber := intent.Number(first); isNumber {
				continue
			}
		}
		target, ok := namedDate(after, today)
		if !ok {
			continue // "30 days from now" is an offset, below
		}
		if w == "since" || w == "from" {
			target = lastOccurrence(after, target, today)
		}
		days := daysBetween(today, target)
		if days < 0 {
			days = -days
		}
		value := strconv.Itoa(days)
		// "weeks until christmas" answers in weeks
		if strings.HasSuffix(before, "weeks") {
			name := plural(days/7, "week")
			if days%7 != 0 {
				name += " " + plural(days%7, "day")
			}
			return Answer{Value: name, Name: name, Hint: target.Format("Mon 2 Jan 2006") + " · " + plural(days, "day")}, true
		}
		return Answer{Value: value, Name: plural(days, "day"), Hint: target.Format("Mon 2 Jan 2006") + weeksHint(days)}, true
	}

	// "in 30 days", "30 days from now", "today + 2 weeks", "10 days ago"
	if d, ok := offset(words); ok {
		target := today.AddDate(d[0], d[1], d[2])
		value := target.Format("2006-01-02")
		return Answer{Value: value, Name: target.Format("Mon 2 Jan 2006"), Hint: value}, true
	}

	// "day is 2026-12-25", "2026-12-25", "day is christmas"
	if rest, ok := strings.CutPrefix(q, "day is "); ok || len(words) == 1 {
		if !ok {
			rest = q
		}
		if t, ok := namedDate(rest, today); ok && (len(words) > 1 || strings.Count(rest, "-") == 2) {
			days := daysBetween(today, t)
			return Answer{Value: t.Format("Monday"), Name: t.Format("Monday, 2 January 2006"), Hint: inDays(days)}, true
		}
	}

	if q == "week number" || q == "week is it" || q == "week no" {
		_, week := now.ISOWeek()
		value := strconv.Itoa(week)
		return Answer{Value: value, Name: "Week " + value, Hint: "ISO week of " + now.Format("2 Jan 2006")}, true
	}

	// a unix timestamp in seconds or milliseconds, 2001 to 2100: a phone
	// number is ten digits too, but starts higher
	if len(words) == 1 && (len(q) == 10 || len(q) == 13) && isDigits(q) {
		n, _ := strconv.ParseInt(q, 10, 64)
		t := time.Unix(n, 0)
		if len(q) == 13 {
			t = time.UnixMilli(n)
		}
		// a phone number is ten digits too: only dates from 2001 to a few
		// years ahead pass as timestamps
		if t.Year() < 2001 || t.Year() > time.Now().Year()+5 {
			return Answer{}, false
		}
		value := t.Format("2006-01-02 15:04:05")
		return Answer{Value: value, Name: value, Hint: "unix time · " + t.UTC().Format("15:04 UTC")}, true
	}
	if q == "unix time" || q == "timestamp" || q == "epoch" || q == "now in unix" {
		value := strconv.FormatInt(now.Unix(), 10)
		return Answer{Value: value, Name: value, Hint: "unix time now"}, true
	}
	return Answer{}, false
}

// daysBetween counts calendar days; rounding absorbs a DST hour.
func daysBetween(from, to time.Time) int {
	return int(math.Round(to.Sub(from).Hours() / 24))
}

func startOfDay(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}

var holidays = map[string][2]int{
	"christmas": {12, 25}, "xmas": {12, 25}, "christmas eve": {12, 24},
	"new year": {1, 1}, "new years": {1, 1}, "new year's": {1, 1}, "new year's eve": {12, 31}, "new years eve": {12, 31},
	"halloween": {10, 31}, "valentine's": {2, 14}, "valentines": {2, 14}, "valentines day": {2, 14},
}

// namedDate reads "christmas" (the next one), "2026-12-25", "25 dec",
// "dec 25", "today", "tomorrow", "yesterday".
func namedDate(s string, today time.Time) (time.Time, bool) {
	s = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(s), " day"))
	// "25th of december," is 25 december
	var parts []string
	for _, w := range strings.Fields(strings.ReplaceAll(s, ",", " ")) {
		if w == "of" {
			continue
		}
		if d, ok := strings.CutSuffix(w, "st"); ok && isDigits(d) {
			w = d
		} else if d, ok := strings.CutSuffix(w, "nd"); ok && isDigits(d) {
			w = d
		} else if d, ok := strings.CutSuffix(w, "rd"); ok && isDigits(d) {
			w = d
		} else if d, ok := strings.CutSuffix(w, "th"); ok && isDigits(d) {
			w = d
		}
		parts = append(parts, w)
	}
	s = strings.Join(parts, " ")
	switch s {
	case "today":
		return today, true
	case "tomorrow":
		return today.AddDate(0, 0, 1), true
	case "yesterday":
		return today.AddDate(0, 0, -1), true
	}
	if md, ok := holidays[s]; ok {
		t := time.Date(today.Year(), time.Month(md[0]), md[1], 0, 0, 0, 0, today.Location())
		if t.Before(today) {
			t = t.AddDate(1, 0, 0)
		}
		return t, true
	}
	for _, layout := range []string{"2006-01-02", "2 Jan 2006", "Jan 2 2006", "2 January 2006", "January 2 2006"} {
		if t, err := time.ParseInLocation(layout, s, today.Location()); err == nil {
			return t, true
		}
	}
	// no year: the next one
	for _, layout := range []string{"2 Jan", "Jan 2", "2 January", "January 2"} {
		if t, err := time.ParseInLocation(layout, s, today.Location()); err == nil {
			t = time.Date(today.Year(), t.Month(), t.Day(), 0, 0, 0, 0, today.Location())
			if t.Before(today) {
				t = t.AddDate(1, 0, 0)
			}
			return t, true
		}
	}
	return time.Time{}, false
}

// lastOccurrence: "days since christmas" or "since 5 jan" means the one
// that has passed, unless a year was given.
func lastOccurrence(s string, t, today time.Time) time.Time {
	for _, w := range strings.Fields(s) {
		if len(w) == 4 && isDigits(w) {
			return t
		}
	}
	if t.After(today) {
		return t.AddDate(-1, 0, 0)
	}
	return t
}

// offset reads "in 30 days", "in two weeks", "30 days from now", "2 weeks
// from tomorrow", "a week ago", "today + 2 weeks" as years, months, days.
func offset(words []string) ([3]int, bool) {
	sign, base := 1, 0
	switch {
	case len(words) >= 3 && words[0] == "in":
		words = words[1:]
	case len(words) >= 4 && words[len(words)-2] == "from":
		switch words[len(words)-1] {
		case "now", "today":
		case "tomorrow":
			base = 1
		default:
			return [3]int{}, false
		}
		words = words[:len(words)-2]
	case len(words) >= 3 && words[len(words)-1] == "ago":
		words, sign = words[:len(words)-1], -1
	case len(words) >= 4 && (words[0] == "today" || words[0] == "tomorrow") && (words[1] == "+" || words[1] == "-"):
		if words[1] == "-" {
			sign = -1
		}
		if words[0] == "tomorrow" {
			base = 1
		}
		words = words[2:]
	default:
		return [3]int{}, false
	}
	// "a couple of weeks" is "couple weeks", "a few days" is "few days"
	var amount []string
	for _, w := range words {
		if w != "of" {
			amount = append(amount, w)
		}
	}
	if len(amount) == 3 && (amount[0] == "a" || amount[0] == "an") {
		amount = amount[1:]
	}
	words = amount
	if len(words) != 2 {
		return [3]int{}, false
	}
	f, ok := intent.Number(words[0])
	if !ok || f != float64(int(f)) {
		return [3]int{}, false
	}
	n := int(f) * sign
	switch strings.TrimSuffix(words[1], "s") {
	case "day":
		return [3]int{0, 0, n + base}, true
	case "week":
		return [3]int{0, 0, 7*n + base}, true
	case "month":
		return [3]int{0, n, base}, true
	case "year":
		return [3]int{n, 0, base}, true
	}
	return [3]int{}, false
}

// plural: "1 day", "2 days".
func plural(n int, word string) string {
	if n == 1 {
		return "1 " + word
	}
	return strconv.Itoa(n) + " " + word + "s"
}

func weeksHint(days int) string {
	if days < 14 {
		return ""
	}
	switch days % 7 {
	case 0:
		return fmt.Sprintf(" · %d weeks", days/7)
	case 1:
		return fmt.Sprintf(" · %d weeks 1 day", days/7)
	}
	return fmt.Sprintf(" · %d weeks %d days", days/7, days%7)
}

func inDays(days int) string {
	switch {
	case days == 0:
		return "today"
	case days == 1:
		return "tomorrow"
	case days == -1:
		return "yesterday"
	case days > 0:
		return fmt.Sprintf("in %d days", days)
	}
	return fmt.Sprintf("%d days ago", -days)
}

func isDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return s != ""
}
