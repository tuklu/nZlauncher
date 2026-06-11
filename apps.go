package main

import (
	"os"
	"path/filepath"
	"strings"
)

// User dir scanned first so it overrides system entries with the same id.
func scanApps() []Action {
	dirs := []string{
		filepath.Join(os.Getenv("HOME"), ".local/share/applications"),
		"/usr/local/share/applications",
		"/usr/share/applications",
	}

	seen := map[string]bool{}
	var apps []Action
	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			id := e.Name()
			if !strings.HasSuffix(id, ".desktop") || seen[id] {
				continue
			}
			seen[id] = true
			if a, ok := parseDesktop(filepath.Join(dir, id), id); ok {
				apps = append(apps, a)
			}
		}
	}
	return apps
}

func parseDesktop(path, id string) (Action, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Action{}, false
	}

	var name, execLine, categories, iconName string
	terminal := false
	inEntry := false
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "[") {
			inEntry = line == "[Desktop Entry]"
			continue
		}
		if !inEntry {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		switch key {
		case "Name":
			if name == "" {
				name = val
			}
		case "Exec":
			if execLine == "" {
				execLine = val
			}
		case "NoDisplay", "Hidden":
			if val == "true" {
				return Action{}, false
			}
		case "Terminal":
			terminal = val == "true"
		case "Categories":
			categories = val
		case "Icon":
			if iconName == "" {
				iconName = val
			}
		}
	}

	if name == "" || execLine == "" {
		return Action{}, false
	}

	cmd := "uwsm-app -- " + stripFieldCodes(execLine)
	if terminal {
		cmd = "xdg-terminal-exec " + stripFieldCodes(execLine)
	}
	icon, keywords := categoryMeta(categories)
	return Action{ID: "app:" + id, Name: name, Icon: icon, IconName: iconName, Group: "app", Keywords: keywords, Cmd: cmd}, true
}

var categories = []struct {
	match    string
	icon     string
	keywords []string
}{
	{"TerminalEmulator", "", []string{"terminal", "term", "shell"}},
	{"WebBrowser", "󰖟", []string{"browser", "web", "internet"}},
	{"IDE", "", []string{"editor", "code", "dev"}},
	{"Development", "", []string{"dev", "code", "programming"}},
	{"Audio", "󰝚", []string{"music", "audio", "sound", "player"}},
	{"AudioVideo", "󰕧", []string{"media", "video", "player"}},
	{"Video", "󰕧", []string{"video", "media", "player"}},
	{"Game", "󰊗", []string{"game", "gaming"}},
	{"Graphics", "󰋩", []string{"image", "photo", "design", "graphics"}},
	{"Office", "󰈙", []string{"office", "document"}},
	{"Settings", "", []string{"settings", "preferences", "config"}},
	{"System", "", []string{"system"}},
	{"Network", "󰖟", []string{"network", "internet"}},
}

// Icon comes from the first matching category, keywords from all of them.
func categoryMeta(list string) (string, []string) {
	icon := "󰀻"
	var keywords []string
	for _, c := range categories {
		if !strings.Contains(list, c.match) {
			continue
		}
		if icon == "󰀻" {
			icon = c.icon
		}
		keywords = append(keywords, c.keywords...)
	}
	return icon, keywords
}

func stripFieldCodes(exec string) string {
	fields := strings.Fields(exec)
	kept := fields[:0]
	for _, f := range fields {
		if len(f) == 2 && f[0] == '%' {
			continue
		}
		kept = append(kept, f)
	}
	return strings.Join(kept, " ")
}
