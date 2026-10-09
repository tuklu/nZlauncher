package providers

import (
	_ "embed"
	"strings"

	"github.com/tuklu/nZlauncher/internal/action"
	"github.com/tuklu/nZlauncher/internal/shell"
)

//go:embed emoji.txt
var emojiData string

// emoji — embedded unicode table, picking copies to clipboard.
func Emoji(q string) []action.Action {
	q = strings.ToLower(q)
	var acts []action.Action
	for _, line := range strings.Split(emojiData, "\n") {
		ch, name, ok := strings.Cut(line, "\t")
		if !ok {
			continue
		}
		if q != "" && !strings.Contains(name, q) {
			continue
		}
		acts = append(acts, action.Action{
			ID: "emoji:" + name, Name: ch + "  " + name, Group: "emoji",
			Cmd: "wl-copy " + shell.Quote(ch),
		})
		if len(acts) == 40 {
			break
		}
	}
	return acts
}
