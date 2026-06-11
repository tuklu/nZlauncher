package main

import (
	"os/exec"
	"strings"
)

// monitor / display — hyprmoncfg profiles; auto-detect picks the right one
// for the current lid/HDMI state, the TUI handles everything deeper.
func monitorActions(q string) []Action {
	q = strings.ToLower(q)
	acts := []Action{{
		ID: "mon:auto", Name: "Monitor: Auto Detect", Icon: "󰍹", Group: "monitor",
		Keywords: []string{"profile", "display"},
		Cmd:      "monitor-switch",
	}}

	out, err := exec.Command("hyprmoncfg", "profiles").Output()
	if err == nil {
		for i, line := range strings.Split(string(out), "\n") {
			fields := strings.Fields(line)
			if i == 0 || len(fields) < 3 { // header row
				continue
			}
			name := fields[0]
			acts = append(acts, Action{
				ID: "mon:" + name, Name: "Monitor Profile: " + name, Icon: "󰍹", Group: "monitor",
				Cmd: "hyprmoncfg apply " + shellQuote(name) + " --confirm-timeout 0",
			})
		}
	}

	if q != "" {
		filtered := acts[:0]
		for _, a := range acts {
			if strings.Contains(strings.ToLower(a.Name), q) {
				filtered = append(filtered, a)
			}
		}
		acts = filtered
	}

	return append(acts, tuiTail("hyprmoncfg", "Hyprmoncfg", "full monitor manager")...)
}
