package main

import "strings"

const (
	scoreNamePrefix  = 100
	scoreWordPrefix  = 85
	scoreNameSub     = 70
	scoreKeyword     = 50
	scoreNameFuzzy   = 40
	scoreKeywordFzy  = 25
	scoreGroupPrefix = 20
	frecencyCap      = 50
	contextCap       = 40
)

// Every token must match somewhere or the action is excluded entirely.
func matchScore(a Action, tokens []string) int {
	name := strings.ToLower(a.Name)
	words := strings.Fields(name)

	total := 0
	for _, t := range tokens {
		best := 0
		switch {
		case strings.HasPrefix(name, t):
			best = scoreNamePrefix
		case wordPrefix(words, t):
			best = scoreWordPrefix
		case strings.Contains(name, t):
			best = scoreNameSub
		case subsequence(name, t):
			best = scoreNameFuzzy
		}

		if best < scoreKeyword {
			for _, k := range a.Keywords {
				k = strings.ToLower(k)
				if strings.HasPrefix(k, t) || strings.Contains(k, t) {
					best = max(best, scoreKeyword)
				} else if subsequence(k, t) {
					best = max(best, scoreKeywordFzy)
				}
			}
		}

		if best == 0 && strings.HasPrefix(a.Group, t) {
			best = scoreGroupPrefix
		}
		if best == 0 {
			return 0
		}
		total += best
	}
	return total
}

func wordPrefix(words []string, t string) bool {
	for _, w := range words {
		if strings.HasPrefix(w, t) {
			return true
		}
	}
	return false
}

func subsequence(s, q string) bool {
	i := 0
	for j := 0; j < len(s) && i < len(q); j++ {
		if s[j] == q[i] {
			i++
		}
	}
	return i == len(q)
}
