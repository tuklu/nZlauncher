package files

import (
	"bytes"
	"path/filepath"
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
	entry int32
	score int32
	mtime int64
	depth int32 // length of the directory path, shallower ranks first
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
	var all []match
	if cands, ok := ix.candidates(f.Terms); ok {
		all = considerList(ix, f, cands)
	} else {
		all = scanAll(ix, f)
	}

	// best match first; among equals the newer and the shallower file
	better := func(a, b match) bool {
		if a.score != b.score {
			return a.score > b.score
		}
		if a.mtime != b.mtime {
			return a.mtime > b.mtime
		}
		return a.depth < b.depth
	}
	// keep the top Limit without sorting every match
	top := make([]match, 0, f.Limit+1)
	for _, m := range all {
		if len(top) == f.Limit && !better(m, top[len(top)-1]) {
			continue
		}
		k := sort.Search(len(top), func(n int) bool { return better(m, top[n]) })
		top = append(top, match{})
		copy(top[k+1:], top[k:])
		top[k] = m
		if len(top) > f.Limit {
			top = top[:f.Limit]
		}
	}
	for _, m := range top {
		e := &ix.entries[m.entry]
		hits = append(hits, Hit{Path: ix.path(e), Size: e.Size, ModTime: time.Unix(e.ModTime, 0)})
	}
	return hits, true
}

func (f Filter) passes(ix *index, i int, dirOK map[int32]bool) bool {
	e := &ix.entries[i]
	if !f.After.IsZero() && e.ModTime < f.After.Unix() || !f.Before.IsZero() && e.ModTime >= f.Before.Unix() {
		return false
	}
	if f.MinSize > 0 && e.Size < f.MinSize || f.MaxSize > 0 && e.Size > f.MaxSize {
		return false
	}
	if len(f.Exts) > 0 {
		if !hasExt(ix.lowerName(i), f.Exts) {
			return false
		}
	}
	if len(f.InDirs) > 0 {
		ok, seen := dirOK[e.dir]
		if !seen {
			for _, d := range f.InDirs {
				if dir := ix.dirs[e.dir]; dir == d || strings.HasPrefix(dir, d+"/") {
					ok = true
				}
			}
			dirOK[e.dir] = ok
		}
		if !ok {
			return false
		}
	}
	return true
}

func hasExt(name string, exts []string) bool {
	ext := strings.TrimPrefix(filepath.Ext(name), ".")
	for _, e := range exts {
		if ext == e {
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
func searchRange(ix *index, f Filter, lo, hi int) []match {
	var hits []match
	dirOK := make(map[int32]bool)
	consider := func(i int) {
		if m, ok := ix.consider(f, i, dirOK); ok {
			hits = append(hits, m)
		}
	}
	if lo >= hi {
		return nil
	}
	if len(f.Terms) == 0 {
		for i := lo; i < hi; i++ {
			consider(i)
		}
		return hits
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
		consider(i)
	}
	return hits
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

func considerList(ix *index, f Filter, list []uint32) []match {
	var hits []match
	dirOK := make(map[int32]bool)
	for _, id := range list {
		if m, ok := ix.consider(f, int(id), dirOK); ok {
			hits = append(hits, m)
		}
	}
	return hits
}

// scanAll is for queries the trigram lists cannot narrow: one slice of the
// entries per core.
func scanAll(ix *index, f Filter) []match {
	parts := runtime.GOMAXPROCS(0)
	results := make([][]match, parts)
	var wg sync.WaitGroup
	for p := range parts {
		lo, hi := p*len(ix.entries)/parts, (p+1)*len(ix.entries)/parts
		wg.Add(1)
		go func() {
			defer wg.Done()
			results[p] = searchRange(ix, f, lo, hi)
		}()
	}
	wg.Wait()
	var all []match
	for _, r := range results {
		all = append(all, r...)
	}
	return all
}

func (ix *index) consider(f Filter, i int, dirOK map[int32]bool) (match, bool) {
	if !f.passes(ix, i, dirOK) {
		return match{}, false
	}
	e := &ix.entries[i]
	score, matched := nameScore(ix.lowerName(i), f.Terms)
	if !matched {
		return match{}, false
	}
	return match{entry: int32(i), score: int32(score), mtime: e.ModTime, depth: int32(len(ix.dirs[e.dir]))}, true
}
