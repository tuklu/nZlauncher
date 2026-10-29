package intent

import (
	"math"
	"strings"
)

// Typos are measured as a weighted edit distance: a letter added, dropped
// or changed costs 1, two neighbours swapped cost 1, and a letter changed
// for the key next to it costs 0.5, since that is how fingers slip.

var keyRows = []string{"qwertyuiop", "asdfghjkl", "zxcvbnm"}

// keyPos places each letter on a QWERTY board, rows staggered like the keys.
var keyPos = func() map[byte][2]float64 {
	stagger := []float64{0, 0.25, 0.75}
	pos := map[byte][2]float64{}
	for r, row := range keyRows {
		for c := range len(row) {
			pos[row[c]] = [2]float64{float64(r), float64(c) + stagger[r]}
		}
	}
	return pos
}()

// adjacent[a][b]: the two letters are neighbouring keys. Built once; a
// lookup per compared letter is all the distance can afford.
var adjacent = func() (t [256][256]bool) {
	for a, pa := range keyPos {
		for b, pb := range keyPos {
			t[a][b] = a != b && math.Hypot(pa[0]-pb[0], pa[1]-pb[1]) < 1.3
		}
	}
	return t
}()

// substitution costs 0.5 for a neighbouring key and far for any other.
func substitution(a, b byte, far float64) float64 {
	switch {
	case a == b:
		return 0
	case adjacent[a][b]:
		return 0.5
	}
	return far
}

// Distance is the weighted optimal string alignment distance of a and b.
func Distance(a, b string) float64 {
	return distance(a, b, math.Inf(1), 1)
}

// distance stops early, returning more than limit, once every alignment
// costs more than limit. far is what changing a letter for a key that is
// not its neighbour costs. Rows live on the stack for words up to 32 letters.
func distance(a, b string, limit, far float64) float64 {
	var r0, r1, r2 [33]float64
	prev2, prev, cur := r0[:], r1[:], r2[:]
	if len(b) > 32 {
		prev2, prev, cur = make([]float64, len(b)+1), make([]float64, len(b)+1), make([]float64, len(b)+1)
	}
	prev2, prev, cur = prev2[:len(b)+1], prev[:len(b)+1], cur[:len(b)+1]
	for j := range prev {
		prev[j] = float64(j)
	}
	for i := 1; i <= len(a); i++ {
		cur[0] = float64(i)
		best := cur[0]
		for j := 1; j <= len(b); j++ {
			d := min(prev[j]+1, cur[j-1]+1, prev[j-1]+substitution(a[i-1], b[j-1], far))
			if i > 1 && j > 1 && a[i-1] == b[j-2] && a[i-2] == b[j-1] {
				d = min(d, prev2[j-2]+1)
			}
			cur[j] = d
			best = min(best, d)
		}
		if best > limit {
			return best
		}
		prev2, prev, cur = prev, cur, prev2
	}
	return prev[len(b)]
}

// Budget is how many edits a word of n letters may carry and still be
// recognised: none under five letters (too many real words sit one edit
// from "wifi" or "file"), one up to seven, two from eight.
func Budget(n int) float64 {
	switch {
	case n < 5:
		return 0
	case n < 8:
		return 1
	}
	return 2
}

// Close reports whether typed is within its budget of want. The first letter
// has to agree, unless the first two are swapped ("ifrefox").
func Close(typed, want string) bool {
	_, ok := closeness(typed, want)
	return ok
}

func closeness(typed, want string) (float64, bool) {
	return closenessWith(typed, want, Budget(len(typed)), 1)
}

// strictBudget is for correcting a word into the grammar's vocabulary,
// where a wrong fix changes what the sentence means: from six letters on,
// one edit up to nine ("workplace" is two from "workspace"), two from ten.
func strictBudget(n int) float64 {
	switch {
	case n < 6:
		return 0
	case n < 10:
		return 1
	}
	return 2
}

func closenessWith(typed, want string, budget, far float64) (float64, bool) {
	if budget == 0 || len(want) == 0 || abs(len(typed)-len(want)) > int(budget) {
		return 0, false
	}
	if typed[0] != want[0] && !(len(typed) > 1 && len(want) > 1 && typed[0] == want[1] && typed[1] == want[0]) {
		return 0, false
	}
	d := distance(typed, want, budget, far)
	return d, d <= budget
}

// Vocab is a word list a typo can be corrected to, bucketed by first
// letter: only words that start like the typed one (or with its second
// letter, for a swap) are worth measuring.
type Vocab struct {
	byFirst [256][]string
	// strict: a far-key letter change is no typo ("order" is not "older",
	// "match" is not "march"), and only words of six letters or more are
	// corrected. Fingers slip onto neighbouring keys, double or drop a
	// letter, or swap two; a far-off letter is a different word.
	strict bool
}

// NewVocab corrects with the plain budget: for names a misspelling is
// more likely than another real word ("celcius").
func NewVocab(words map[string]bool) *Vocab {
	return newVocab(words, false)
}

// NewStrictVocab is for the grammar's own words, see Vocab.strict.
func NewStrictVocab(words map[string]bool) *Vocab {
	return newVocab(words, true)
}

func newVocab(words map[string]bool, strict bool) *Vocab {
	v := &Vocab{strict: strict}
	for w := range words {
		if len(w) >= 4 {
			v.byFirst[w[0]] = append(v.byFirst[w[0]], w)
		}
	}
	return v
}

// Correct finds the one word in vocab closest to typed within its budget;
// a tie between the closest is no fix.
func (v *Vocab) Correct(typed string) (string, bool) {
	budget, far := Budget(len(typed)), 1.0
	if v.strict {
		budget, far = strictBudget(len(typed)), budget+1
	}
	if budget == 0 {
		return "", false
	}
	best, bestDist, tie := "", math.Inf(1), false
	try := func(words []string) {
		for _, w := range words {
			// a word and its plural or other form are close but not a
			// typo: "config" is not "configs", "saved" is not "saver"
			if stem(w) == stem(typed) {
				continue
			}
			d, ok := closenessWith(typed, w, budget, far)
			switch {
			case !ok:
			case d < bestDist:
				best, bestDist, tie = w, d, false
			case d == bestDist && w != best:
				tie = true
			}
		}
	}
	try(v.byFirst[typed[0]])
	if typed[1] != typed[0] {
		try(v.byFirst[typed[1]])
	}
	return best, best != "" && !tie
}

// stem drops a common ending: saved, saver, saves and save are one word.
func stem(w string) string {
	for _, suffix := range []string{"ing", "ed", "er", "es", "s", "d", "r"} {
		if len(w) > len(suffix)+2 {
			if s, ok := strings.CutSuffix(w, suffix); ok {
				return strings.TrimSuffix(s, "e")
			}
		}
	}
	return strings.TrimSuffix(w, "e")
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
