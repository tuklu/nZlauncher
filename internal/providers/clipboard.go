package providers

import (
	"os/exec"
	"strings"

	"github.com/tuklu/nZlauncher/internal/action"
	"github.com/tuklu/nZlauncher/internal/shell"
)

// clip / clipboard — history via cliphist; picking re-copies the entry.
func Clipboard(q string) []action.Action {
	q = strings.ToLower(q)
	out, err := exec.Command("cliphist", "list").Output()
	if err != nil {
		return nil
	}

	var acts []action.Action
	for _, line := range strings.Split(strings.TrimRight(string(out), "\n"), "\n") {
		id, preview, ok := strings.Cut(line, "\t")
		if !ok {
			continue
		}
		if q != "" && !strings.Contains(strings.ToLower(preview), q) {
			continue
		}
		display := []rune(strings.TrimSpace(preview))
		if len(display) > 70 {
			display = append(display[:67], []rune("...")...)
		}
		acts = append(acts, action.Action{
			ID: "clip", Name: string(display), Icon: "󰅍", Group: "clipboard",
			Cmd: "printf %s " + shell.Quote(id) + " | cliphist decode | wl-copy",
		})
		if len(acts) == 30 {
			break
		}
	}
	return acts
}
