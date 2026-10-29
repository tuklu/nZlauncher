package files

import "strings"

// Scoring borrows from fzf: where a term lands in the name matters more than
// that it lands at all. The name's start beats a word start (after a
// separator or at a capital inside a word), which beats anywhere; a name
// that is the term plus an extension beats all of those; a term found only
// in the folder path counts for a little, and a short name for a little
// more than a long one.
const (
	scoreMatch    = 20
	bonusStart    = 10
	bonusBoundary = 6
	bonusCamel    = 5
	bonusStem     = 12
	scorePath     = 6
	bonusPathPart = 3
	// a term at the start of the name or of a word in it: strong enough for
	// the file to show among apps in a plain search
	strongTerm = scoreMatch + bonusBoundary
)

func isSeparator(b byte) bool {
	return b == ' ' || b == '_' || b == '-' || b == '.' || b == '/'
}

func isUpper(b byte) bool { return b >= 'A' && b <= 'Z' }
func isLower(b byte) bool { return b >= 'a' && b <= 'z' }

// termScore is the best place t lands in the name: lname lowercased, name as
// stored (for capitals). 0 when it is not in the name.
func termScore(lname, name, t string) int {
	best := 0
	for off := 0; off < len(lname); {
		k := strings.Index(lname[off:], t)
		if k < 0 {
			break
		}
		i := off + k
		s := scoreMatch
		switch {
		case i == 0:
			s += bonusStart
		case isSeparator(lname[i-1]):
			s += bonusBoundary
		case len(name) == len(lname) && isUpper(name[i]) && isLower(name[i-1]):
			s += bonusCamel
		}
		best = max(best, s)
		off = i + 1
	}
	if best > 0 {
		stem := lname
		if dot := strings.LastIndexByte(lname, '.'); dot > 0 {
			stem = lname[:dot]
		}
		if stem == t {
			best += bonusStem
		}
	}
	return best
}

// pathScore: t in the lowercased folder path, more at a folder's start.
func pathScore(ldir, t string) int {
	i := strings.Index(ldir, t)
	switch {
	case i < 0:
		return 0
	case i == 0 || ldir[i-1] == '/' || ldir[i-1] == '.':
		return scorePath + bonusPathPart
	}
	return scorePath
}

// fuzzyScore is for names that hold the term's letters in order but not
// together ("cfgtoml" in "config.toml"): a point per letter, more for
// letters that follow each other or start a word, less the wider the
// letters are spread. Always below a real substring match. 0 when the
// letters are not all there.
func fuzzyScore(lname, t string) int {
	best := 0
	// try each place the first letter appears, the first few are enough
	for start, tries := 0, 0; tries < 8; tries++ {
		k := strings.IndexByte(lname[start:], t[0])
		if k < 0 {
			break
		}
		first := start + k
		score, prev, j := 0, -2, 0
		for i := first; i < len(lname) && j < len(t); i++ {
			if lname[i] != t[j] {
				continue
			}
			score += 2
			if i == prev+1 {
				score += 3
			}
			if i == 0 || isSeparator(lname[i-1]) {
				score += 4
			}
			prev, j = i, j+1
		}
		if j == len(t) {
			score -= (prev - first + 1 - len(t)) / 2 // the spread
			best = max(best, score)
		}
		start = first + 1
	}
	return min(best, scoreMatch-1)
}

// scoreEntry scores entry i against every term: each has to land in the
// name or, failing that, in its folder path, and at least one in the name.
// fuzzy lets a term land with its letters in order. strong is whether every
// term landed at the start of the name or of a word in it.
func (ix *index) scoreEntry(i int, terms []string, fuzzy bool) (score int, strong, ok bool) {
	e := &ix.entries[i]
	lname, name := ix.lowerName(i), ix.origName(i)
	inName, strong := false, true
	for _, t := range terms {
		s := termScore(lname, name, t)
		switch {
		case s > 0:
			inName = true
			strong = strong && s >= strongTerm
		case pathScore(ix.lowerDirs[e.dir], t) > 0:
			s, strong = pathScore(ix.lowerDirs[e.dir], t), false
		case fuzzy && len(t) >= 3:
			if s = fuzzyScore(lname, t); s == 0 {
				return 0, false, false
			}
			inName, strong = true, false
		default:
			return 0, false, false
		}
		score += s
	}
	if !inName {
		return 0, false, false
	}
	return score - min(len(lname)/12, 4), strong, true
}
