package files

import (
	"bytes"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
)

// Filter is a file query. Every set field must hold; Terms must all appear
// in the file name.
type Filter struct {
	Terms   []string // lowercase
	Exts    []string // lowercase, without the dot
	After   time.Time
	Before  time.Time
	MinSize int64
	MaxSize int64
	InDirs  []string // only under these (absolute) directories
	Limit   int
}

func (f Filter) Empty() bool {
	return len(f.Terms) == 0 && len(f.Exts) == 0 && f.After.IsZero() && f.Before.IsZero() &&
		f.MinSize == 0 && f.MaxSize == 0 && len(f.InDirs) == 0
}

type Hit struct {
	Path    string
	Size    int64
	ModTime time.Time
}

// match is a hit before its path is built: only the winners get one.
type match struct {
	entry  int32
	score  int32
	mtime  int64
	depth  int32 // length of the directory path, shallower ranks first
	buried bool
}

// Search answers from the index; ok is false when it is not built yet.
func Search(f Filter) (hits []Hit, ok bool) {
	ix := current.Load()
	if ix == nil {
		return nil, false
	}
	if f.Limit == 0 {
		f.Limit = 25
	}
	// walk the smallest set that holds every match: a trigram list, the
	// files of the asked types, or the run of entries under the folders
	lo, hi := 0, len(ix.entries)
	if len(f.InDirs) > 0 {
		lo, hi = ix.span(f.InDirs)
	}
	byType := -1
	if len(f.Exts) > 0 {
		byType = 0
		for _, ext := range f.Exts {
			byType += len(ix.byExt[ext])
		}
	}
	c := newCollector(ix, f)
	if cands, ok := ix.candidates(f.Terms); ok && len(cands) <= max(hi-lo, 0) {
		considerList(c, cands)
	} else if byType >= 0 && byType < hi-lo {
		for _, ext := range f.Exts {
			considerList(c, ix.byExt[ext])
		}
	} else if hi-lo < len(ix.entries) && hi-lo <= splitAbove {
		searchRange(c, lo, hi)
	} else if hi-lo < len(ix.entries) {
		inParallel(c, hi-lo, func(part *collector, a, b int) { searchRange(part, lo+a, lo+b) })
	} else {
		scanAll(c)
	}

	for _, m := range c.top {
		e := &ix.entries[m.entry]
		hits = append(hits, Hit{Path: ix.path(e), Size: e.Size, ModTime: time.Unix(e.ModTime, 0)})
	}
	return hits, true
}

func (f Filter) passes(ix *index, i int, dirOK []int8) bool {
	e := &ix.entries[i]
	if !f.After.IsZero() && e.ModTime < f.After.Unix() || !f.Before.IsZero() && e.ModTime >= f.Before.Unix() {
		return false
	}
	if f.MinSize > 0 && e.Size < f.MinSize || f.MaxSize > 0 && e.Size > f.MaxSize {
		return false
	}
	if len(f.Exts) > 0 && !hasExt(ix.nameBytes(i), f.Exts) {
		return false
	}
	if len(f.InDirs) > 0 {
		if dirOK[e.dir] == 0 {
			dirOK[e.dir] = -1
			for _, d := range f.InDirs {
				if dir := ix.dirs[e.dir]; dir == d || strings.HasPrefix(dir, d+"/") {
					dirOK[e.dir] = 1
				}
			}
		}
		if dirOK[e.dir] < 0 {
			return false
		}
	}
	return true
}

// hasExt checks the packed name in place: a query with no terms tests every
// entry, and a string per entry would cost more than the test.
func hasExt(name []byte, exts []string) bool {
	dot := bytes.LastIndexByte(name, '.')
	if dot < 0 {
		return false
	}
	for _, e := range exts {
		if string(name[dot+1:]) == e {
			return true
		}
	}
	return false
}

// A term at the start of the name beats one at a word start, which beats one
// anywhere. No terms matches everything equally.
func nameScore(name string, terms []string) (int, bool) {
	score := 0
	for _, t := range terms {
		i := strings.Index(name, t)
		switch {
		case i < 0:
			return 0, false
		case i == 0:
			score += 3
		case strings.ContainsRune(" _-.", rune(name[i-1])):
			score += 2
		default:
			score++
		}
	}
	return score, true
}

// searchRange finds matches among entries [lo, hi).
func searchRange(c *collector, lo, hi int) {
	ix, f := c.ix, c.f
	if lo >= hi {
		return
	}
	if len(f.Terms) == 0 {
		for i := lo; i < hi; i++ {
			c.consider(i)
		}
		return
	}

	// the longest term has the fewest matches; find it in the packed names,
	// then check everything else on those candidates only
	first := f.Terms[0]
	for _, t := range f.Terms[1:] {
		if len(t) > len(first) {
			first = t
		}
	}
	needle := []byte(first)
	// scan for the needle's rarest byte, not its first: memchr then stops
	// rarely, and each stop is checked against the whole needle
	anchor := 0
	for k := range needle {
		if ix.freq[needle[k]] < ix.freq[needle[anchor]] {
			anchor = k
		}
	}
	start := int(ix.offsets[lo])
	end := len(ix.names)
	if hi < len(ix.offsets) {
		end = int(ix.offsets[hi])
	}
	last := -1 // entry already considered
	for pos := start + anchor; pos < end; {
		k := bytes.IndexByte(ix.names[pos:end], needle[anchor])
		if k < 0 {
			break
		}
		at := pos + k - anchor
		pos += k + 1
		if at < start || at+len(needle) > end || !bytes.Equal(ix.names[at:at+len(needle)], needle) {
			continue
		}
		// the needle has no newline, so a match lies inside one name
		i := sort.Search(hi-lo, func(n int) bool { return int(ix.offsets[lo+n]) > at }) + lo - 1
		if i == last {
			continue
		}
		last = i
		c.consider(i)
	}
}

// candidates is the shortest trigram list among the terms' trigrams: every
// match is in it. Terms under three letters have no trigram to narrow by.
func (ix *index) candidates(terms []string) ([]uint32, bool) {
	var best []uint32
	found := false
	for _, t := range terms {
		for k := 0; k+3 <= len(t); k++ {
			span := ix.trigrams[trigram(t[k:])]
			list := ix.postings[span[0] : span[0]+span[1]]
			if !found || len(list) < len(best) {
				best, found = list, true
			}
		}
	}
	return best, found
}

// Lists and ranges this long are split across cores.
const splitAbove = 20000

func considerList(c *collector, list []uint32) {
	if len(list) > splitAbove {
		inParallel(c, len(list), func(part *collector, lo, hi int) { considerList(part, list[lo:hi]) })
		return
	}
	for _, id := range list {
		c.consider(int(id))
	}
}

// scanAll is for queries nothing narrows.
func scanAll(c *collector) {
	inParallel(c, len(c.ix.entries), func(part *collector, lo, hi int) { searchRange(part, lo, hi) })
}

// inParallel runs search over [0, n) in one slice per core, each slice with
// its own top, and merges the tops into c.
func inParallel(c *collector, n int, search func(part *collector, lo, hi int)) {
	parts := runtime.GOMAXPROCS(0)
	results := make([]*collector, parts)
	var wg sync.WaitGroup
	for p := range parts {
		results[p] = newCollector(c.ix, c.f)
		wg.Add(1)
		go func() {
			defer wg.Done()
			search(results[p], p*n/parts, (p+1)*n/parts)
		}()
	}
	wg.Wait()
	for _, r := range results {
		for _, m := range r.top {
			c.add(m)
		}
	}
}

// collector keeps the best Limit matches as they are found, so a query that
// matches 100k files never holds 100k of them.
type collector struct {
	ix    *index
	f     Filter
	dirOK []int8 // per directory: 0 not checked yet, 1 inside InDirs, -1 outside
	top   []match
}

func newCollector(ix *index, f Filter) *collector {
	c := &collector{ix: ix, f: f, top: make([]match, 0, f.Limit+1)}
	if len(f.InDirs) > 0 {
		c.dirOK = make([]int8, len(ix.dirs))
	}
	return c
}

func (c *collector) consider(i int) {
	ix, f := c.ix, c.f
	if !f.passes(ix, i, c.dirOK) {
		return
	}
	e := &ix.entries[i]
	score := 0
	if len(f.Terms) > 0 {
		var matched bool
		if score, matched = nameScore(ix.lowerName(i), f.Terms); !matched {
			return
		}
	}
	c.add(match{entry: int32(i), score: int32(score), mtime: e.ModTime, depth: int32(len(ix.dirs[e.dir])), buried: ix.buried[e.dir]})
}

// best match first; among equals the user's own, the newer, the shallower file
func better(a, b match) bool {
	if a.score != b.score {
		return a.score > b.score
	}
	if a.buried != b.buried {
		return b.buried
	}
	if a.mtime != b.mtime {
		return a.mtime > b.mtime
	}
	return a.depth < b.depth
}

func (c *collector) add(m match) {
	if len(c.top) == c.f.Limit && !better(m, c.top[len(c.top)-1]) {
		return
	}
	k := sort.Search(len(c.top), func(n int) bool { return better(m, c.top[n]) })
	c.top = append(c.top, match{})
	copy(c.top[k+1:], c.top[k:])
	c.top[k] = m
	if len(c.top) > c.f.Limit {
		c.top = c.top[:c.f.Limit]
	}
}
