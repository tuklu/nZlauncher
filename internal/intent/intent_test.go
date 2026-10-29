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
		"launch firefox":            {"open", "firefox"},
		"disable bluetoth":          {"off", "bluetooth"},
		"louder":                    {"volume", "up"},
		"brigtness 40 percent":      {"brightness", "40", "%"},
		"fire":                      {"fire"}, // short words are never corrected
		"screnshots yesterdy":       {"screenshots", "yesterday"},
		"turn on do not disturb":    {"turn", "on", "dnd"},
		"keep the screen on for 2h": {"awake", "for", "2h"},
		"Power Saver mode":          {"power-saver", "mode"},
		"caffeine":                  {"awake"},
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
		{q: "pdfs from last week", sure: true, ext: "pdf", after: date(2026, 10, 19), before: date(2026, 10, 26)},
		{q: "pdf last year", sure: true, ext: "pdf", after: date(2025, 1, 1), before: date(2026, 1, 1)},
		{q: "pdfs from september 2025", sure: true, ext: "pdf", after: date(2025, 9, 1), before: date(2025, 10, 1)},
		{q: "pdfs from the past week", sure: true, ext: "pdf", after: now.Add(-7 * 24 * time.Hour)},
		{q: "pdfs this month", sure: true, ext: "pdf", after: date(2026, 10, 1)},
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

		{q: "pdfs between 1mb and 5mb", sure: true, ext: "pdf", minSize: 1 << 20, maxSize: 5 << 20},
		{q: "videos 1-5gb", sure: true, ext: "mp4", minSize: 1 << 30, maxSize: 5 << 30},
		{q: "images 200kb-2mb", sure: true, ext: "png", minSize: 200 << 10, maxSize: 2 << 20},
		{q: "pdfs 1 to 5 mb", sure: true, ext: "pdf", minSize: 1 << 20, maxSize: 5 << 20},
		{q: "pdfs from june to august", sure: true, ext: "pdf", after: date(2026, 6, 1), before: date(2026, 9, 1)},
		{q: "pdfs between march and may 2025", sure: true, ext: "pdf", after: date(2025, 3, 1), before: date(2025, 6, 1)},
		{q: "pdfs from 2024 to 2025", sure: true, ext: "pdf", after: date(2024, 1, 1), before: date(2026, 1, 1)},
		{q: "report pdf 2026-10-01", sure: true, terms: []string{"report"}, ext: "pdf", after: date(2026, 10, 1), before: date(2026, 10, 2)},
		{q: "images from 4 jan to 5 march", sure: true, ext: "png", after: date(2026, 1, 4), before: date(2026, 3, 6)},
		{q: "pdfs from 4th of january to march 5th", sure: true, ext: "pdf", after: date(2026, 1, 4), before: date(2026, 3, 6)},
		{q: "pdfs from 4 dec to 5 jan", sure: true, ext: "pdf", after: date(2025, 12, 4), before: date(2026, 1, 6)},
		{q: "pdfs on 4 jan 2025", sure: true, ext: "pdf", after: date(2025, 1, 4), before: date(2025, 1, 5)},
		{q: "pdfs since jan 4, 2025", sure: true, ext: "pdf", after: date(2025, 1, 4)},
		// from the edge hunt
		{q: "pdfs from september to november", sure: true, ext: "pdf", after: date(2025, 9, 1), before: date(2025, 12, 1)},
		{q: "pdfs from august to december", sure: true, ext: "pdf", after: date(2025, 8, 1), before: date(2026, 1, 1)},
		{q: "pdfs from june to now", sure: true, ext: "pdf", after: date(2026, 6, 1)},
		{q: "pdfs from 2024 to 2027", sure: true, ext: "pdf", after: date(2024, 1, 1), before: date(2028, 1, 1)},
		{q: "pdfs 2024-2025", sure: true, ext: "pdf", after: date(2024, 1, 1), before: date(2026, 1, 1)},
		{q: "pdfs at most 5mb", sure: true, ext: "pdf", maxSize: 5 << 20},
		{q: "pdfs 5mb or less", sure: true, ext: "pdf", maxSize: 5 << 20},
		{q: "pdfs up to 5mb", sure: true, ext: "pdf", maxSize: 5 << 20},
		{q: "pdfs no more than 5mb", sure: true, ext: "pdf", maxSize: 5 << 20},
		{q: "pdfs at least 5mb", sure: true, ext: "pdf", minSize: 5 << 20},
		{q: "pdfs bigger than 5 megabytes", sure: true, ext: "pdf", minSize: 5 << 20},
		{q: "pdfs bigger than 5mb?", sure: true, ext: "pdf", minSize: 5 << 20},
		{q: "pdf, today", sure: true, ext: "pdf", after: date(2026, 10, 29)},
		{q: "pdfs in my projects", sure: true, ext: "pdf", dir: "Projects"},
		{q: "invoice in my documents", sure: true, terms: []string{"invoice"}, dir: "Documents"},
		{q: "pdfs in the downloads folder", sure: true, ext: "pdf", dir: "Downloads"},
		{q: "today's screenshots", sure: true, ext: "png", after: date(2026, 10, 29), dir: "Pictures/Screenshots"},
		{q: "pdfs last thursday", sure: true, ext: "pdf", after: date(2026, 10, 22), before: date(2026, 10, 23)},
		{q: "pdfs from last october", sure: true, ext: "pdf", after: date(2025, 10, 1), before: date(2025, 11, 1)},
		{q: "pdfs from two weeks ago", sure: true, ext: "pdf", after: now.Add(-21 * 24 * time.Hour), before: now.Add(-7 * 24 * time.Hour)},
		{q: "pdfs from a couple of weeks ago", sure: true, ext: "pdf", after: now.Add(-21 * 24 * time.Hour), before: now.Add(-7 * 24 * time.Hour)},
		{q: "pdfs from the past few days", sure: true, ext: "pdf", after: now.Add(-3 * 24 * time.Hour)},
		{q: "order pdfs", sure: true, terms: []string{"order"}, ext: "pdf"},
		{q: "match photos", sure: true, terms: []string{"match"}, ext: "jpg"},
		{q: "screenshot", sure: false, ext: "png"}, // the capture action
		{q: "firefox", sure: false, terms: []string{"firefox"}},
		{q: "find invoice", sure: false, terms: []string{"invoice"}},
		{q: "open music", sure: false, ext: "mp3"},
		{q: "files", sure: false},
		{q: "nzlauncher config", sure: false, terms: []string{"nzlauncher", "config"}},
		{q: "configs in projects", sure: true, ext: "toml", dir: "Projects"},
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

func TestTypos(t *testing.T) {
	vocab := map[string]bool{"bluetooth": true, "brightness": true, "screenshots": true, "yesterday": true,
		"celsius": true, "fahrenheit": true, "firefox": true, "files": true, "nightlight": true}
	cases := map[string]string{
		"bluetoth":    "bluetooth",   // dropped letter
		"blurtooth":   "bluetooth",   // neighbour key
		"brigthness":  "brightness",  // swap
		"brihgtnes":   "brightness",  // two slips in a long word
		"scrrenshots": "screenshots", // extra letter
		"celcius":     "celsius",     // c for s
		"farenheit":   "fahrenheit",
		"ifrefox":     "firefox", // first two swapped
		"nightlihgt":  "nightlight",
		"fire":        "", // too short to be a typo of anything
		"xylophone":   "",
		"filed":       "", // files with a d, not a typo
	}
	for typed, want := range cases {
		got, ok := NewVocab(vocab).Correct(typed)
		if !ok {
			got = ""
		}
		if got != want {
			t.Errorf("%q: got %q, want %q", typed, got, want)
		}
	}
	if d := Distance("wifi", "wifu"); d != 0.5 {
		t.Errorf("neighbour key should cost 0.5, got %v", d)
	}
}

func BenchmarkWords(b *testing.B) {
	for _, q := range []string{"chromium", "pdfs from september 2025", "44 degree celcius in fahrenheit"} {
		b.Run(q, func(b *testing.B) {
			for b.Loop() {
				Words(q)
			}
		})
	}
}

func TestStrictVocabulary(t *testing.T) {
	// real slips are still fixed
	for typed, want := range map[string]string{
		"bluetoth": "bluetooth", "brigthness": "brightness", "yesterdy": "yesterday",
		"screnshots": "screenshots", "blurtooth": "bluetooth", "nightlihgt": "nightlight",
	} {
		if got, _ := vocab.Correct(typed); got != want {
			t.Errorf("%q: got %q, want %q", typed, got, want)
		}
	}
	// ordinary words the edge hunt saw turned into grammar words
	for _, w := range []string{"order", "saved", "saves", "changed", "charge", "chance", "marsh",
		"stars", "smart", "stark", "times", "tiger", "rewind", "unfair", "loader", "dimmed",
		"internal", "interest", "sneaker", "audit", "aware", "alter", "fills", "filed", "latent",
		"resent", "never", "mouth", "workplace", "sundae", "launcher", "trucks", "boots", "ducks",
		"slider"} {
		if got, ok := vocab.Correct(w); ok {
			t.Errorf("%q was corrected to %q", w, got)
		}
	}
}
