package files

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"
)

func TestFindsAndFilters(t *testing.T) {
	root := t.TempDir()
	mk := func(rel string, size int, age time.Duration) {
		p := filepath.Join(root, rel)
		os.MkdirAll(filepath.Dir(p), 0o755)
		os.WriteFile(p, make([]byte, size), 0o644)
		when := time.Now().Add(-age)
		os.Chtimes(p, when, when)
	}
	mk("Invoice-March.pdf", 10, 2*24*time.Hour)
	mk("docs/invoice-old.pdf", 10, 90*24*time.Hour)
	mk("docs/invoice.txt", 10, time.Hour)
	mk("node_modules/invoice.pdf", 10, time.Hour)
	mk("big.mp4", 6<<20, time.Hour)
	current.Store(build(Config{Roots: []string{root}, Exclude: []string{"node_modules"}}))

	names := func(f Filter) []string {
		hits, _ := Search(f)
		var out []string
		for _, h := range hits {
			out = append(out, filepath.Base(h.Path))
		}
		return out
	}
	if got := names(Filter{Terms: []string{"invoice"}}); len(got) != 3 {
		t.Fatalf("excluded dir leaked or a match missing: %v", got)
	}
	if got := names(Filter{Terms: []string{"invoice"}, Exts: []string{"pdf"}, After: time.Now().Add(-7 * 24 * time.Hour)}); len(got) != 1 || got[0] != "Invoice-March.pdf" {
		t.Fatalf("type+date filter: %v", got)
	}
	if got := names(Filter{MinSize: 5 << 20}); len(got) != 1 || got[0] != "big.mp4" {
		t.Fatalf("size filter: %v", got)
	}
}

// go test ./internal/files -run '^$' -bench . : against the real home directory
func BenchmarkHome(b *testing.B) {
	t0 := time.Now()
	ix := build(DefaultConfig())
	b.Logf("built %d files in %d dirs in %v", len(ix.entries), len(ix.dirs), time.Since(t0))
	current.Store(ix)
	home := os.Getenv("HOME")
	img := []string{"png", "jpg", "jpeg", "webp"}
	for name, f := range map[string]Filter{
		"cat":               {Terms: []string{"cat"}},
		"invoice-pdf":       {Terms: []string{"invoice"}, Exts: []string{"pdf"}},
		"today":             {After: time.Now().Add(-24 * time.Hour)},
		"pdfs":              {Exts: []string{"pdf"}},
		"screenshots-today": {Exts: img, InDirs: []string{home + "/Pictures/Screenshots"}, After: time.Now().Add(-24 * time.Hour)},
		"images-downloads":  {Exts: img, InDirs: []string{home + "/Downloads"}},
		"go-in-development": {Exts: []string{"go"}, InDirs: []string{home + "/Development"}},
	} {
		b.Run(name, func(b *testing.B) {
			for b.Loop() {
				Search(f)
			}
		})
	}
}

func TestFolderSpanExtListsHiddenLast(t *testing.T) {
	root := t.TempDir()
	mk := func(rel string, age time.Duration) {
		p := filepath.Join(root, rel)
		os.MkdirAll(filepath.Dir(p), 0o755)
		os.WriteFile(p, []byte("x"), 0o644)
		when := time.Now().Add(-age)
		os.Chtimes(p, when, when)
	}
	mk("a/one.png", time.Hour)
	mk("a/sub/two.png", 2*time.Hour)
	mk("a/zz.txt", time.Hour)
	mk("b/three.png", time.Minute)
	mk(".cache-ish/newest.png", time.Second)
	mk("proj/build/gen.png", 2*time.Second)
	current.Store(build(Config{Roots: []string{root}}))

	names := func(f Filter) []string {
		hits, _ := Search(f)
		var out []string
		for _, h := range hits {
			out = append(out, filepath.Base(h.Path))
		}
		return out
	}
	if got := names(Filter{Exts: []string{"png"}, InDirs: []string{filepath.Join(root, "a")}}); !slices.Equal(got, []string{"one.png", "two.png"}) {
		t.Fatalf("folder span: %v", got)
	}
	// newest first, but dot folders and build trees after the user's files
	if got := names(Filter{Exts: []string{"png"}}); !slices.Equal(got, []string{"three.png", "one.png", "two.png", "newest.png", "gen.png"}) {
		t.Fatalf("ext list order: %v", got)
	}
}
