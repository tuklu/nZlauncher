package files

import (
	"os"
	"path/filepath"
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
	for _, f := range []Filter{{Terms: []string{"cat"}}, {Terms: []string{"invoice"}, Exts: []string{"pdf"}}, {After: time.Now().Add(-24 * time.Hour)}} {
		b.Run(fmtFilter(f), func(b *testing.B) {
			for b.Loop() {
				Search(f)
			}
		})
	}
}

func fmtFilter(f Filter) string {
	if len(f.Terms) > 0 {
		return f.Terms[0]
	}
	return "today"
}
