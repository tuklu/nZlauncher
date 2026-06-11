package main

import (
	_ "embed"
	"strings"
)

//go:embed emoji.txt
var emojiData string

// emoji — embedded unicode table, picking copies to clipboard.
func emojiActions(q string) []Action {
	q = strings.ToLower(q)
	var acts []Action
	for _, line := range strings.Split(emojiData, "\n") {
		ch, name, ok := strings.Cut(line, "\t")
		if !ok {
			continue
		}
		if q != "" && !strings.Contains(name, q) {
			continue
		}
		acts = append(acts, Action{
			ID: "emoji:" + name, Name: ch + "  " + name, Group: "emoji",
			Cmd: "wl-copy " + shellQuote(ch),
		})
		if len(acts) == 40 {
			break
		}
	}
	return acts
}
