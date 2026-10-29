package intent

import "math"

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

func substitution(a, b byte) float64 {
	switch {
	case a == b:
		return 0
	case adjacent[a][b]:
		return 0.5
	}
	return 1
}

// Distance is the weighted optimal string alignment distance of a and b.
func Distance(a, b string) float64 {
	return distance(a, b, math.Inf(1))
}

// distance stops early, returning more than limit, once every alignment
// costs more than limit. Rows live on the stack for words up to 32 letters.
func distance(a, b string, limit float64) float64 {
	var r0, r1, r2 [33]float64
	if len(b) > 32 {
		return math.Inf(1)
	}
	prev2, prev, cur := r0[:len(b)+1], r1[:len(b)+1], r2[:len(b)+1]
	for j := range prev {
		prev[j] = float64(j)
	}
	for i := 1; i <= len(a); i++ {
		cur[0] = float64(i)
		best := cur[0]
		for j := 1; j <= len(b); j++ {
			d := min(prev[j]+1, cur[j-1]+1, prev[j-1]+substitution(a[i-1], b[j-1]))
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
	budget := Budget(len(typed))
	if budget == 0 || len(want) == 0 || abs(len(typed)-len(want)) > int(budget) {
		return 0, false
	}
	if typed[0] != want[0] && !(len(typed) > 1 && len(want) > 1 && typed[0] == want[1] && typed[1] == want[0]) {
		return 0, false
	}
	d := distance(typed, want, budget)
	return d, d <= budget
}

// Vocab is a word list a typo can be corrected to, bucketed by first
// letter: only words that start like the typed one (or with its second
// letter, for a swap) are worth measuring.
type Vocab struct {
	byFirst [256][]string
}

func NewVocab(words map[string]bool) *Vocab {
	v := &Vocab{}
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
	if Budget(len(typed)) == 0 {
		return "", false
	}
	best, bestDist, tie := "", math.Inf(1), false
	try := func(words []string) {
		for _, w := range words {
			d, ok := closeness(typed, w)
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

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
