package main

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

// keys / binds — every Hyprland binding, searchable; picking one executes it.
func keysActions(q string) []Action {
	out, err := exec.Command("hyprctl", "binds", "-j").Output()
	if err != nil {
		return nil
	}
	var binds []struct {
		Modmask     int    `json:"modmask"`
		Key         string `json:"key"`
		Description string `json:"description"`
		Dispatcher  string `json:"dispatcher"`
		Arg         string `json:"arg"`
		Submap      string `json:"submap"`
	}
	if json.Unmarshal(out, &binds) != nil {
		return nil
	}

	var acts []Action
	for _, b := range binds {
		if b.Submap != "" || b.Key == "" {
			continue
		}

		var mods []string
		for _, m := range []struct {
			bit  int
			name string
		}{{64, "SUPER"}, {8, "ALT"}, {4, "CTRL"}, {1, "SHIFT"}} {
			if b.Modmask/m.bit%2 == 1 {
				mods = append(mods, m.name)
			}
		}
		combo := b.Key
		if len(mods) > 0 {
			combo = strings.Join(mods, "+") + " + " + b.Key
		}

		what := b.Description
		if what == "" {
			if b.Dispatcher == "__lua" {
				what = "(lua action)"
			} else {
				what = strings.TrimSpace(b.Dispatcher + " " + b.Arg)
			}
		}
		name := fmt.Sprintf("%-28s %s", combo, what)

		if q != "" && !strings.Contains(strings.ToLower(name), q) {
			continue
		}

		cmd := "true"
		// __lua callbacks can't be dispatched via hyprctl CLI; view-only
		if b.Dispatcher != "" && b.Dispatcher != "__lua" {
			cmd = "hyprctl dispatch " + b.Dispatcher + " " + shellQuote(b.Arg)
		}
		acts = append(acts, Action{
			ID: "key:" + combo, Name: name, Icon: "󰌌", Group: "keys", Cmd: cmd,
		})
	}
	return acts
}
