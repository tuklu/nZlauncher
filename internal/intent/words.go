// Package intent reads a sentence the way a person writes it: any word
// order, synonyms, small typos, and the numbers, sizes, dates and file kinds
// inside it. A deterministic grammar, no model, so every answer is the same
// and costs microseconds.
package intent

import "strings"

// Word is one token of the query. Canon is what the grammar matches on,
// lowercased; Raw is what the user typed, for arguments and paths.
type Word struct {
	Raw    string
	Canon  string
	Filler bool
}

// synonyms map a word to its canonical form; a value of two words expands
// into both ("louder" is "volume up").
var synonyms = map[string]string{
	"launch": "open", "run": "open", "start": "open", "exec": "open",
	"change": "set", "make": "set", "adjust": "set", "put": "set",
	"enable": "on", "activate": "on", "disable": "off", "deactivate": "off",
	"google": "search", "lookup": "search", "web": "search",
	"locate": "find",
	"pair":   "connect", "unpair": "disconnect",
	"goto": "go", "jump": "go",
	"timer": "remind", "remember": "remind", "reminder": "remind", "alarm": "remind",
	"stop": "kill", "quit": "kill",

	"sound": "volume", "vol": "volume",
	"backlight": "brightness", "bright": "brightness",
	"wlan": "wifi", "wireless": "wifi", "wi-fi": "wifi", "internet": "wifi", "network": "wifi",
	"bt":         "bluetooth",
	"background": "wallpaper", "bg": "wallpaper",
	"ws":      "workspace",
	"display": "monitor",
	"audio":   "output", "speaker": "output", "speakers": "output",

	"louder": "volume up", "quieter": "volume down", "softer": "volume down",
	"brighter": "brightness up", "dimmer": "brightness down", "darker": "brightness down",
	"mute": "volume mute", "unmute": "volume unmute", "silence": "volume mute",
	"increase": "up", "raise": "up", "higher": "up",
	"decrease": "down", "lower": "down", "reduce": "down",
	"percent": "%",

	"caffeine": "awake", "saver": "power-saver", "powersave": "power-saver",
}

var fillerWords = map[string]bool{
	"the": true, "a": true, "an": true, "my": true, "please": true, "to": true,
	"for": true, "of": true, "in": true, "me": true, "all": true, "some": true,
}

// vocabulary is every word the grammar knows; only these are typo-corrected.
var vocabulary = func() map[string]bool {
	v := map[string]bool{}
	for k, val := range synonyms {
		v[k] = true
		for _, w := range strings.Fields(val) {
			v[w] = true
		}
	}
	for _, w := range []string{
		"open", "set", "turn", "switch", "go", "focus", "connect", "disconnect", "search",
		"find", "show", "list", "kill", "remind", "files", "file",
		"today", "yesterday", "recent", "recently", "latest", "newest", "week", "month",
		"year", "hours", "minutes", "older", "newer", "bigger", "larger", "smaller",
		"since", "before", "after",
		"dnd", "nightlight", "awake", "powermode", "performance", "balanced",
	} {
		v[w] = true
	}
	for w := range kinds {
		v[w] = true
	}
	for _, w := range weekdays {
		v[w] = true
	}
	for _, w := range months {
		v[w] = true
	}
	return v
}()

// phrases are names of more than one word; each becomes one token.
var phrases = []struct {
	words []string
	canon string
}{
	{[]string{"do", "not", "disturb"}, "dnd"},
	{[]string{"focus", "mode"}, "dnd"},
	{[]string{"night", "light"}, "nightlight"},
	{[]string{"night", "mode"}, "nightlight"},
	{[]string{"keep", "the", "screen", "on"}, "awake"},
	{[]string{"keep", "screen", "on"}, "awake"},
	{[]string{"keep", "awake"}, "awake"},
	{[]string{"stay", "awake"}, "awake"},
	{[]string{"power", "profile"}, "powermode"},
	{[]string{"power", "mode"}, "powermode"},
	{[]string{"power", "saver"}, "power-saver"},
	{[]string{"power", "saving"}, "power-saver"},
	{[]string{"battery", "saver"}, "power-saver"},
}

// joinPhrases folds the phrases among the typed tokens into single tokens.
func joinPhrases(typed []string) []string {
	var out []string
	for i := 0; i < len(typed); i++ {
		joined := false
		for _, p := range phrases {
			n := len(p.words)
			if i+n > len(typed) {
				continue
			}
			match := true
			for k, w := range p.words {
				if strings.ToLower(typed[i+k]) != w {
					match = false
					break
				}
			}
			if match {
				out = append(out, "\x00"+p.canon+"\x00"+strings.Join(typed[i:i+n], " "))
				i += n - 1
				joined = true
				break
			}
		}
		if !joined {
			out = append(out, typed[i])
		}
	}
	return out
}

// Words splits q into tokens with phrases, synonyms and typos resolved.
func Words(q string) []Word {
	var out []Word
	var tokens []string
	for _, t := range strings.Fields(q) {
		// "5mb?", "pdf," "today's": the mark is not part of the word
		if t = strings.TrimRight(t, ",?!;:"); len(t) > 1 {
			t = strings.TrimSuffix(t, ".")
		}
		if t != "" {
			tokens = append(tokens, t)
		}
	}
	for _, typed := range joinPhrases(tokens) {
		if canon, raw, ok := strings.Cut(strings.TrimPrefix(typed, "\x00"), "\x00"); ok && strings.HasPrefix(typed, "\x00") {
			out = append(out, Word{Raw: raw, Canon: canon})
			continue
		}
		tok := strings.ToLower(typed)
		if s, ok := strings.CutSuffix(tok, "'s"); ok {
			tok = s
		} else if s, ok := strings.CutSuffix(tok, "’s"); ok {
			tok = s
		}
		canon := tok
		if !vocabulary[tok] {
			canon = correct(tok)
		}
		if s, ok := synonyms[canon]; ok {
			canon = s
		}
		for i, part := range strings.Fields(canon) {
			raw := typed
			if i > 0 {
				raw = ""
			}
			out = append(out, Word{Raw: raw, Canon: part, Filler: fillerWords[part]})
		}
	}
	return out
}

// Content drops the filler words.
func Content(ws []Word) []Word {
	var out []Word
	for _, w := range ws {
		if !w.Filler {
			out = append(out, w)
		}
	}
	return out
}

// RawJoin is the typed text of ws, for an argument.
func RawJoin(ws []Word) string {
	var parts []string
	for _, w := range ws {
		if w.Raw != "" {
			parts = append(parts, w.Raw)
		}
	}
	return strings.Join(parts, " ")
}

var vocab = NewStrictVocab(vocabulary)

// correct fixes a typo in a known word, see Vocab.Correct.
func correct(tok string) string {
	if fix, ok := vocab.Correct(tok); ok {
		return fix
	}
	return tok
}
