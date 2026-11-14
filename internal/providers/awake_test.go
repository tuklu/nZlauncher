package providers

import (
	"testing"
	"time"

	"github.com/tuklu/nZlauncher/internal/intent"
)

func TestParseAwake(t *testing.T) {
	cases := map[string]struct {
		mode string // "" for any
		d    time.Duration
		set  bool
		off  bool
	}{
		"":                {},
		"2h":              {d: 2 * time.Hour, set: true},
		"90 minutes":      {d: 90 * time.Minute, set: true},
		"sleep 2 hours":   {mode: "sleep", d: 2 * time.Hour, set: true},
		"all forever":     {mode: "all", set: true},
		"screen 45m":      {mode: "screen", d: 45 * time.Minute, set: true},
		"off":             {off: true},
		"sleep off":       {mode: "sleep", off: true},
		"everything 1h":   {mode: "all", d: time.Hour, set: true},
		"lid closed 30m":  {mode: "sleep", d: 30 * time.Minute, set: true},
		"with lid closed": {mode: "sleep"},
	}
	for q, want := range cases {
		got := parseAwake(q)
		mode := ""
		if got.mode != nil {
			mode = got.mode.key
		}
		if mode != want.mode || got.d != want.d || got.set != want.set || got.off != want.off {
			t.Errorf("%q: got mode=%q d=%v set=%v off=%v, want %+v", q, mode, got.d, got.set, got.off, want)
		}
	}
}

func TestClockUntil(t *testing.T) {
	now := time.Date(2026, 11, 14, 14, 0, 0, 0, time.Local)
	cases := map[string]struct {
		d    time.Duration
		used int
	}{
		"18:30":   {4*time.Hour + 30*time.Minute, 1},
		"6pm":     {4 * time.Hour, 1},
		"6 pm":    {4 * time.Hour, 2},
		"6.30pm":  {4*time.Hour + 30*time.Minute, 1},
		"9am":     {19 * time.Hour, 1}, // already past: tomorrow
		"12am":    {10 * time.Hour, 1},
		"12pm":    {22 * time.Hour, 1},
		"14:00":   {24 * time.Hour, 1}, // now: the next one
		"5":       {0, 0},              // a bare number is not a time
		"25:00":   {0, 0},
		"13pm":    {0, 0},
		"tonight": {0, 0},
	}
	for q, want := range cases {
		d, used := clockUntil(intent.Words(q), now)
		if d != want.d || used != want.used {
			t.Errorf("%q: got %v/%d, want %v/%d", q, d, used, want.d, want.used)
		}
	}
}
