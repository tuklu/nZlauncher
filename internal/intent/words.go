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
	"mute": "volume mute", "unmute": "volume mute",
	"increase": "up", "raise": "up", "higher": "up",
	"decrease": "down", "lower": "down", "reduce": "down",
	"percent": "%",
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

// Words splits q into tokens with synonyms and typos resolved.
func Words(q string) []Word {
	var out []Word
	for _, typed := range strings.Fields(q) {
		tok := strings.ToLower(typed)
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

// correct fixes one typo in a known word. Short words are left alone (too
// many real words sit one edit from "wifi" or "file"), the first letter must
// agree, and an ambiguous fix is no fix.
func correct(tok string) string {
	if len(tok) < 5 {
		return tok
	}
	fix := ""
	for w := range vocabulary {
		if w[0] == tok[0] && len(w) >= 4 && Near(tok, w) {
			if fix != "" {
				return tok
			}
			fix = w
		}
	}
	if fix == "" {
		return tok
	}
	return fix
}

// Near reports whether a and b are at most one edit apart: one letter
// changed, added, dropped, or two neighbours swapped.
func Near(a, b string) bool {
	if a == b {
		return true
	}
	if len(a) > len(b) {
		a, b = b, a
	}
	switch len(b) - len(a) {
	case 0:
		i := 0
		for i < len(a) && a[i] == b[i] {
			i++
		}
		if a[i+1:] == b[i+1:] {
			return true
		}
		return i+1 < len(a) && a[i] == b[i+1] && a[i+1] == b[i] && a[i+2:] == b[i+2:]
	case 1:
		i := 0
		for i < len(a) && a[i] == b[i] {
			i++
		}
		return a[i:] == b[i+1:]
	}
	return false
}
