package intent

import (
	"strconv"
	"strings"
	"time"
)

// Slots are the pieces of a sentence that carry values. Each parser looks at
// ws[i:] and reports how many words it used, 0 for none.

var durationUnits = map[string]time.Duration{
	"s": time.Second, "sec": time.Second, "secs": time.Second, "second": time.Second, "seconds": time.Second,
	"m": time.Minute, "min": time.Minute, "mins": time.Minute, "minute": time.Minute, "minutes": time.Minute,
	"h": time.Hour, "hr": time.Hour, "hrs": time.Hour, "hour": time.Hour, "hours": time.Hour,
	"d": day, "day": day, "days": day,
	"w": week, "wk": week, "wks": week, "week": week, "weeks": week,
	"mo": month, "month": month, "months": month,
	"y": year, "yr": year, "yrs": year, "year": year, "years": year,
}

const (
	day   = 24 * time.Hour
	week  = 7 * day
	month = 30 * day
	year  = 365 * day
)

// Number reads "5", "2.5", "a", "an", "half".
func Number(s string) (float64, bool) {
	switch s {
	case "a", "an", "one":
		return 1, true
	case "half":
		return 0.5, true
	}
	n, err := strconv.ParseFloat(s, 64)
	return n, err == nil && n >= 0
}

// splitNumber cuts "10m" into 10 and "m".
func splitNumber(s string) (float64, string, bool) {
	k := 0
	for k < len(s) && (s[k] >= '0' && s[k] <= '9' || s[k] == '.') {
		k++
	}
	if k == 0 {
		return 0, "", false
	}
	n, err := strconv.ParseFloat(s[:k], 64)
	return n, s[k:], err == nil
}

// Duration reads "10m", "1h30m", "10 minutes", "an hour", "half an hour".
func Duration(ws []Word, i int) (time.Duration, int) {
	if i >= len(ws) {
		return 0, 0
	}
	if d, ok := compactDuration(ws[i].Canon); ok {
		return d, 1
	}
	// "half an hour"
	if ws[i].Canon == "half" && i+2 < len(ws) && (ws[i+1].Canon == "an" || ws[i+1].Canon == "a") {
		if unit, ok := durationUnits[ws[i+2].Canon]; ok {
			return unit / 2, 3
		}
	}
	if i+1 < len(ws) {
		if n, ok := Number(ws[i].Canon); ok {
			if unit, ok := durationUnits[ws[i+1].Canon]; ok {
				return time.Duration(n * float64(unit)), 2
			}
		}
	}
	return 0, 0
}

// compactDuration reads "10m", "90s", "1h30m", "2d".
func compactDuration(s string) (time.Duration, bool) {
	var total time.Duration
	for s != "" {
		n, rest, ok := splitNumber(s)
		if !ok {
			return 0, false
		}
		k := 0
		for k < len(rest) && rest[k] >= 'a' && rest[k] <= 'z' {
			k++
		}
		unit, ok := durationUnits[rest[:k]]
		if !ok || k == 0 {
			return 0, false
		}
		total += time.Duration(n * float64(unit))
		s = rest[k:]
	}
	return total, total > 0
}

var sizeUnits = map[string]int64{
	"b": 1, "byte": 1, "bytes": 1,
	"k": 1 << 10, "kb": 1 << 10, "kib": 1 << 10,
	"mb": 1 << 20, "mib": 1 << 20, "meg": 1 << 20, "megs": 1 << 20,
	"g": 1 << 30, "gb": 1 << 30, "gib": 1 << 30, "gig": 1 << 30, "gigs": 1 << 30,
	"tb": 1 << 40, "tib": 1 << 40,
}

// Size reads "5mb", "5 mb", "1.5gb". loose also takes "5m" as megabytes,
// for after a word like "bigger" where it cannot mean minutes.
func Size(ws []Word, i int, loose bool) (int64, int) {
	if i >= len(ws) {
		return 0, 0
	}
	unitOf := func(u string) (int64, bool) {
		if loose && u == "m" {
			return 1 << 20, true
		}
		n, ok := sizeUnits[u]
		return n, ok
	}
	if n, rest, ok := splitNumber(ws[i].Canon); ok && rest != "" {
		if unit, ok := unitOf(rest); ok {
			return int64(n * float64(unit)), 1
		}
		return 0, 0
	}
	if n, err := strconv.ParseFloat(ws[i].Canon, 64); err == nil && i+1 < len(ws) {
		if unit, ok := unitOf(ws[i+1].Canon); ok {
			return int64(n * float64(unit)), 2
		}
	}
	return 0, 0
}

var weekdays = []string{"sunday", "monday", "tuesday", "wednesday", "thursday", "friday", "saturday"}
var months = []string{"january", "february", "march", "april", "may", "june", "july",
	"august", "september", "october", "november", "december"}

// Weekdays only by full name: "sun" and "sat" are too often something else.
func weekdayOf(s string) (time.Weekday, bool) {
	for k, name := range weekdays {
		if s == name {
			return time.Weekday(k), true
		}
	}
	return 0, false
}

// Months also by short name ("sept", "oct"), except "mar".
func monthOf(s string) (time.Month, bool) {
	for k, name := range months {
		if s == name || len(s) >= 3 && len(s) <= 4 && strings.HasPrefix(name, s) && s != "mar" {
			return time.Month(k + 1), true
		}
	}
	return 0, false
}

// Span is a time range; a zero end is open.
type Span struct{ From, To time.Time }

func startOfDay(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}

func startOfWeek(t time.Time) time.Time {
	back := (int(t.Weekday()) + 6) % 7 // weeks start on Monday
	return startOfDay(t).AddDate(0, 0, -back)
}

func startOfMonth(t time.Time) time.Time {
	y, m, _ := t.Date()
	return time.Date(y, m, 1, 0, 0, 0, 0, t.Location())
}

func startOfYear(t time.Time) time.Time {
	return time.Date(t.Year(), 1, 1, 0, 0, 0, 0, t.Location())
}

// When reads a moment or a period: "today", "yesterday", "this week",
// "last month" (from the start of the previous month on), "past 3 days",
// "2 weeks ago", "monday", "october", "2026-10-01", "recent" (past week).
func When(ws []Word, i int, now time.Time) (Span, int) {
	if i >= len(ws) {
		return Span{}, 0
	}
	today := startOfDay(now)
	w := ws[i].Canon
	switch w {
	case "today":
		return Span{From: today}, 1
	case "yesterday":
		return Span{today.AddDate(0, 0, -1), today}, 1
	case "recent", "recently":
		return Span{From: now.Add(-week)}, 1
	case "this", "last", "past":
		if i+1 >= len(ws) {
			return Span{}, 0
		}
		next := ws[i+1].Canon
		// "last 3 days", "past 2 weeks": rolling
		if w != "this" {
			if d, n := Duration(ws, i+1); n > 0 {
				return Span{From: now.Add(-d)}, 1 + n
			}
		}
		start := map[string]func(time.Time) time.Time{
			"day": startOfDay, "week": startOfWeek, "month": startOfMonth, "year": startOfYear,
		}[next]
		if start == nil {
			if next == "hour" && w != "this" {
				return Span{From: now.Add(-time.Hour)}, 2
			}
			return Span{}, 0
		}
		from := start(now)
		if w != "this" {
			// "last week" reaches back to the start of the previous week,
			// so it covers both readings: the week before and the past seven days
			from = start(from.Add(-time.Nanosecond))
		}
		return Span{From: from}, 2
	}
	// "3 days ago"
	if d, n := Duration(ws, i); n > 0 && i+n < len(ws) && ws[i+n].Canon == "ago" {
		at := now.Add(-d)
		if d >= day && d%day == 0 && d < week {
			return Span{startOfDay(at), startOfDay(at).AddDate(0, 0, 1)}, n + 1
		}
		return Span{at.Add(-d / 2), at.Add(d / 2)}, n + 1
	}
	if wd, ok := weekdayOf(w); ok {
		back := (int(now.Weekday()) - int(wd) + 7) % 7
		d := today.AddDate(0, 0, -back)
		return Span{d, d.AddDate(0, 0, 1)}, 1
	}
	if m, ok := monthOf(w); ok {
		y := now.Year()
		if m > now.Month() {
			y--
		}
		from := time.Date(y, m, 1, 0, 0, 0, 0, now.Location())
		return Span{from, from.AddDate(0, 1, 0)}, 1
	}
	if t, err := time.ParseInLocation("2006-01-02", w, now.Location()); err == nil {
		return Span{t, t.AddDate(0, 0, 1)}, 1
	}
	return Span{}, 0
}

// Year reads "2025" as that calendar year, for after "in", "from", "since".
func Year(w string, now time.Time) (Span, bool) {
	n, err := strconv.Atoi(w)
	if err != nil || len(w) != 4 || n < 1970 || n > now.Year() {
		return Span{}, false
	}
	from := time.Date(n, 1, 1, 0, 0, 0, 0, now.Location())
	return Span{from, from.AddDate(1, 0, 0)}, true
}
