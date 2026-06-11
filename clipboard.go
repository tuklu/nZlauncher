package main

import (
	"os/exec"
	"strings"
)

// clip / clipboard — history via cliphist; picking re-copies the entry.
func clipboardActions(q string) []Action {
	q = strings.ToLower(q)
	out, err := exec.Command("cliphist", "list").Output()
	if err != nil {
		return nil
	}

	var acts []Action
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
		acts = append(acts, Action{
			ID: "clip", Name: string(display), Icon: "󰅍", Group: "clipboard",
			Cmd: "printf %s " + shellQuote(id) + " | cliphist decode | wl-copy",
		})
		if len(acts) == 30 {
			break
		}
	}
	return acts
}
