package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Open windows join the regular ranking pool: typing an app name offers
// "switch to it" alongside "launch another". Frecency keys on class, not
// address, so the habit of switching to an app survives window churn.
func windowActions() []Action {
	out, err := exec.Command("hyprctl", "clients", "-j").Output()
	if err != nil {
		return nil
	}
	var clients []struct {
		Address string `json:"address"`
		Class   string `json:"class"`
		Title   string `json:"title"`
		Mapped  bool   `json:"mapped"`
	}
	if json.Unmarshal(out, &clients) != nil {
		return nil
	}

	var acts []Action
	for _, c := range clients {
		if !c.Mapped || c.Title == "" || c.Class == "nz.launcher" {
			continue
		}
		title := []rune(c.Title)
		if len(title) > 60 {
			title = append(title[:57], []rune("...")...)
		}
		acts = append(acts, Action{
			ID:       "win:" + c.Class,
			Name:     string(title),
			Icon:     "󱂬",
			Group:    "window",
			Keywords: []string{c.Class, "window", "switch", "focus"},
			Cmd:      "hyprctl dispatch focuswindow address:" + c.Address,
		})
	}
	return acts
}

// kill [name] — user processes sorted by CPU, filtered by name.
func killActions(q string) []Action {
	out, err := exec.Command("ps", "-u", os.Getenv("USER"),
		"-o", "pid,pcpu,pmem,comm", "--sort=-pcpu", "--no-headers").Output()
	if err != nil {
		return nil
	}

	var acts []Action
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}
		pid, cpu, mem, comm := fields[0], fields[1], fields[2], fields[3]
		if q != "" && !strings.Contains(strings.ToLower(comm), q) {
			continue
		}
		acts = append(acts, Action{
			ID:    "kill:" + comm,
			Name:  fmt.Sprintf("Kill %s   (pid %s, cpu %s%%, mem %s%%)", comm, pid, cpu, mem),
			Icon:  "󰚌",
			Group: "kill",
			Cmd:   "kill " + pid,
		})
		if len(acts) == 25 {
			break
		}
	}
	return append(acts, tuiTail("btop", "Btop", "full process monitor")...)
}

// wallpaper [name] — images in ~/.config/neZha/wallpaper, sets via the
// wallpaper script which also persists the choice for next boot.
func wallpaperActions(q string) []Action {
	dir := filepath.Join(os.Getenv("HOME"), ".config/neZha/wallpaper")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	var acts []Action
	for _, e := range entries {
		name := e.Name()
		ext := strings.ToLower(filepath.Ext(name))
		if strings.HasPrefix(name, ".") || ext != ".jpg" && ext != ".jpeg" && ext != ".png" && ext != ".webp" {
			continue
		}
		if q != "" && !strings.Contains(strings.ToLower(name), q) {
			continue
		}
		acts = append(acts, Action{
			ID:    "wp:" + name,
			Name:  "Wallpaper: " + name,
			Icon:  "󰸉",
			Group: "wallpaper",
			Cmd:   "wallpaper " + shellQuote(filepath.Join(dir, name)),
		})
	}
	return acts
}
