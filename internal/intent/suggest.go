package intent

import (
	"sort"
	"strings"
)

// weakWords are the stems of phrase words that say how, not what: "keep" or
// "keeping" alone do not point at keep awake more than at an app called Keep.
var weakWords = func() map[string]bool {
	stems := map[string]bool{}
	for _, w := range []string{"keep", "stay", "mode", "turn", "back", "allow", "never", "block", "prevent", "power"} {
		stems[stem(w)] = true
	}
	return stems
}()

// Suggest names the phrase intents a query comes close to without matching
// one, best first: "unlock" or "screen stay on" are about keep awake. A
// phrase counts when the query's words cover at least half of it, one of
// them a word that means something on its own.
func Suggest(q string) []string {
	var typed []string
	for _, t := range strings.Fields(q) {
		if t = phraseToken(strings.TrimRight(t, ",?!;:.")); t != "" && !fillerWords[t] {
			typed = append(typed, t)
		}
	}
	if len(typed) == 0 {
		return nil
	}
	best := map[string]float64{}
	for _, p := range phrases {
		hits, strong := 0, false
		for _, w := range p.words {
			if fillerWords[w] {
				continue
			}
			for _, t := range typed {
				if looseWord(t, w) {
					hits++
					strong = strong || (len(w) >= 4 && !weakWords[stem(w)])
					break
				}
			}
		}
		if !strong || hits*2 < len(typed) {
			continue
		}
		if score := float64(hits) / float64(len(p.words)); score > best[p.canon] {
			best[p.canon] = score
		}
	}
	canons := make([]string, 0, len(best))
	for c := range best {
		canons = append(canons, c)
	}
	sort.Slice(canons, func(i, j int) bool {
		if best[canons[i]] != best[canons[j]] {
			return best[canons[i]] > best[canons[j]]
		}
		return canons[i] < canons[j]
	})
	return canons
}
