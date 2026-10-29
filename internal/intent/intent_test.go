package intent

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"
)

// Thursday 29 October 2026, 15:00
var now = time.Date(2026, 10, 29, 15, 0, 0, 0, time.Local)

func date(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 0, 0, 0, 0, time.Local)
}

func TestWordsResolveSynonymsAndTypos(t *testing.T) {
	cases := map[string][]string{
		"launch firefox":       {"open", "firefox"},
		"disable bluetoth":     {"off", "bluetooth"},
		"louder":               {"volume", "up"},
		"brigtness 40 percent": {"brightness", "40", "%"},
		"fire":                 {"fire"}, // short words are never corrected
		"screnshots yesterdy":  {"screenshots", "yesterday"},
	}
	for q, want := range cases {
		var got []string
		for _, w := range Words(q) {
			got = append(got, w.Canon)
		}
		if !slices.Equal(got, want) {
			t.Errorf("%q: got %v, want %v", q, got, want)
		}
	}
}

func TestNear(t *testing.T) {
	for _, c := range []struct {
		a, b string
		near bool
	}{
		{"firefox", "fierfox", true}, {"firefox", "firefo", true}, {"firefox", "firefoxx", true},
		{"firefox", "firafox", true}, {"firefox", "fiarfox", false}, {"abc", "xyz", false},
	} {
		if Near(c.a, c.b) != c.near {
			t.Errorf("Near(%q, %q) = %v", c.a, c.b, !c.near)
		}
	}
}

func TestDuration(t *testing.T) {
	cases := map[string]time.Duration{
		"10m": 10 * time.Minute, "1h30m": 90 * time.Minute, "10 minutes": 10 * time.Minute,
		"an hour": time.Hour, "half an hour": 30 * time.Minute, "2 days": 48 * time.Hour,
	}
	for q, want := range cases {
		if got, _ := Duration(Words(q), 0); got != want {
			t.Errorf("%q: got %v, want %v", q, got, want)
		}
	}
}

func TestFiles(t *testing.T) {
	home := t.TempDir()
	for _, d := range []string{"Downloads", "Documents", "Pictures/Screenshots", "Projects", "Code/MyApp"} {
		os.MkdirAll(filepath.Join(home, d), 0o755)
	}
	t.Setenv("HOME", home)

	cases := []struct {
		q       string
		sure    bool
		terms   []string
		ext     string // one of the extensions expected
		after   time.Time
		before  time.Time
		minSize int64
		maxSize int64
		dir     string
	}{
		{q: "pdfs from last week", sure: true, ext: "pdf", after: date(2026, 10, 19)},
		{q: "screenshots today", sure: true, ext: "png", after: date(2026, 10, 29), dir: "Pictures/Screenshots"},
		{q: "invoice docs bigger than 5mb", sure: true, terms: []string{"invoice"}, ext: "pdf", minSize: 5 << 20},
		{q: "videos > 1gb", sure: true, ext: "mkv", minSize: 1 << 30},
		{q: "images in downloads yesterday", sure: true, ext: "jpg", after: date(2026, 10, 28), before: date(2026, 10, 29), dir: "Downloads"},
		{q: "go files in projects", sure: true, ext: "go", dir: "Projects"},
		{q: "recent files", sure: true, after: now.Add(-7 * 24 * time.Hour)},
		{q: "report pdf since monday", sure: true, terms: []string{"report"}, ext: "pdf", after: date(2026, 10, 26)},
		{q: "music older than 1 year", sure: true, ext: "mp3", before: now.Add(-365 * 24 * time.Hour)},
		{q: "small pngs from october", sure: true, ext: "png", after: date(2026, 10, 1), before: date(2026, 11, 1), maxSize: 1 << 20},
		{q: "tax pdf from 2025", sure: true, terms: []string{"tax"}, ext: "pdf", after: date(2025, 1, 1), before: date(2026, 1, 1)},
		{q: "Invoice in Documents", sure: true, terms: []string{"invoice"}, dir: "Documents"},
		{q: "configs in ~/Code/MyApp", sure: true, ext: "toml", dir: "Code/MyApp"},
		{q: "latest screenshot", sure: true, ext: "png", dir: "Pictures/Screenshots"},

		{q: "screenshot", sure: false, ext: "png"}, // the capture action
		{q: "firefox", sure: false, terms: []string{"firefox"}},
		{q: "find invoice", sure: false, terms: []string{"invoice"}},
		{q: "open music", sure: false, ext: "mp3"},
		{q: "files", sure: false},
	}
	for _, c := range cases {
		fq := Files(c.q, now)
		f := fq.Filter
		if fq.Sure != c.sure {
			t.Errorf("%q: sure %v, want %v", c.q, fq.Sure, c.sure)
		}
		if !slices.Equal(f.Terms, c.terms) {
			t.Errorf("%q: terms %v, want %v", c.q, f.Terms, c.terms)
		}
		if c.ext != "" && !slices.Contains(f.Exts, c.ext) {
			t.Errorf("%q: exts %v, want %s among them", c.q, f.Exts, c.ext)
		}
		if !f.After.Equal(c.after) || !f.Before.Equal(c.before) {
			t.Errorf("%q: span %v..%v, want %v..%v", c.q, f.After, f.Before, c.after, c.before)
		}
		if f.MinSize != c.minSize || f.MaxSize != c.maxSize {
			t.Errorf("%q: size %d..%d, want %d..%d", c.q, f.MinSize, f.MaxSize, c.minSize, c.maxSize)
		}
		if want := filepath.Join(home, c.dir); c.dir != "" && !slices.Contains(f.InDirs, want) {
			t.Errorf("%q: dirs %v, want %s", c.q, f.InDirs, want)
		}
	}
}
