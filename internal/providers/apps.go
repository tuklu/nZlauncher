package providers

import (
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/tuklu/nZlauncher/internal/action"
	"github.com/tuklu/nZlauncher/internal/icons"
)

// AppDirs are scanned in this order, user first.
func AppDirs() []string {
	return []string{
		filepath.Join(os.Getenv("HOME"), ".local/share/applications"),
		"/usr/local/share/applications",
		"/usr/share/applications",
	}
}

// Apps scans .desktop entries. The user dir is scanned first so it
// overrides system entries with the same id.
func Apps(lib *icons.Library) []action.Action {
	if !On("apps") {
		return nil
	}
	seen := map[string]bool{}
	var apps []action.Action
	for _, dir := range AppDirs() {
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
			if a, ok := parseDesktop(filepath.Join(dir, id), id, lib); ok {
				apps = append(apps, a)
			}
		}
	}
	return apps
}

func parseDesktop(path, id string, lib *icons.Library) (action.Action, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return action.Action{}, false
	}

	var name, genericName, execLine, categoryList, iconName, wmClass string
	var desktopKeywords []string
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
		case "GenericName":
			if genericName == "" {
				genericName = val
			}
		case "Keywords":
			if desktopKeywords == nil {
				desktopKeywords = strings.FieldsFunc(val, func(r rune) bool { return r == ';' })
			}
		case "Exec":
			if execLine == "" {
				execLine = val
			}
		case "NoDisplay", "Hidden":
			if val == "true" {
				return action.Action{}, false
			}
		case "Terminal":
			terminal = val == "true"
		case "Categories":
			categoryList = val
		case "StartupWMClass":
			wmClass = val
		case "Icon":
			if iconName == "" {
				iconName = val
			}
		}
	}

	if name == "" || execLine == "" {
		return action.Action{}, false
	}

	cmd := "uwsm-app -- " + stripFieldCodes(execLine)
	if terminal {
		cmd = "xdg-terminal-exec " + stripFieldCodes(execLine)
	}
	desktopCategories := strings.FieldsFunc(categoryList, func(r rune) bool { return r == ';' })
	keywords := categoryKeywords(desktopCategories)
	keywords = append(keywords, desktopKeywords...)
	if genericName != "" {
		keywords = append(keywords, genericName)
	}
	icon, found := lib.AppGlyph(strings.TrimSuffix(id, ".desktop"), iconName, wmClass, execBinary(execLine))
	if !found {
		icon = lib.CategoryGlyph(byPriority(desktopCategories))
	}
	return action.Action{ID: "app:" + id, Name: name, Icon: icon, IconName: iconName, Group: "app", Keywords: keywords, Cmd: cmd}, true
}

// Order is priority: the first match picks the category icon, so specific
// categories sit above broad ones like Network or Office.
var categories = []struct {
	match    string
	keywords []string
}{
	{"TerminalEmulator", []string{"terminal", "term", "shell"}},
	{"Email", []string{"email", "mail", "inbox"}},
	{"InstantMessaging", []string{"chat", "messaging", "im"}},
	{"Chat", []string{"chat", "messaging"}},
	{"FileManager", []string{"files", "file manager", "explorer"}},
	{"TextEditor", []string{"editor", "text", "notes"}},
	{"Spreadsheet", []string{"spreadsheet", "sheets", "excel"}},
	{"WordProcessor", []string{"document", "writer", "word"}},
	{"Presentation", []string{"presentation", "slides"}},
	{"WebBrowser", []string{"browser", "web", "internet"}},
	{"IDE", []string{"editor", "code", "dev"}},
	{"Development", []string{"dev", "code", "programming"}},
	{"Audio", []string{"music", "audio", "sound", "player"}},
	{"AudioVideo", []string{"media", "video", "player"}},
	{"Video", []string{"video", "media", "player"}},
	{"Game", []string{"game", "gaming"}},
	{"Graphics", []string{"image", "photo", "design", "graphics"}},
	{"Office", []string{"office", "document"}},
	{"Settings", []string{"settings", "preferences", "config"}},
	{"System", []string{"system"}},
	{"Network", []string{"network", "internet"}},
}

func categoryKeywords(desktopCategories []string) []string {
	var keywords []string
	for _, c := range categories {
		if slices.Contains(desktopCategories, c.match) {
			keywords = append(keywords, c.keywords...)
		}
	}
	return keywords
}

// Table order wins over the desktop file's own order, so Network;Email;
// resolves to Email. Categories the table doesn't know come after.
func byPriority(desktopCategories []string) []string {
	var ordered []string
	for _, c := range categories {
		if slices.Contains(desktopCategories, c.match) {
			ordered = append(ordered, c.match)
		}
	}
	for _, c := range desktopCategories {
		if !slices.Contains(ordered, c) {
			ordered = append(ordered, c)
		}
	}
	return ordered
}

// "env FOO=1 /usr/lib/thunderbird/thunderbird %u" -> "thunderbird"
func execBinary(execLine string) string {
	for _, f := range strings.Fields(execLine) {
		if f == "env" || strings.Contains(f, "=") {
			continue
		}
		return filepath.Base(f)
	}
	return ""
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
