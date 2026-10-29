package providers

import (
	"encoding/xml"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/tuklu/nZlauncher/internal/action"
	"github.com/tuklu/nZlauncher/internal/shell"
)

// file / f — fd search under home; bare query shows recently used files.
func Files(q string) []action.Action {
	if !On("files") {
		return nil
	}
	if q == "" {
		return recentFiles(15)
	}

	args := []string{"--type", "f", "--hidden", "--max-results", "40"}
	for _, ex := range []string{
		".git", ".cache", "go/pkg", "node_modules", ".cargo", ".rustup",
		".npm", ".local/share", ".local/state", ".mozilla", ".gradle",
		".java", "__pycache__", ".venv",
	} {
		args = append(args, "--exclude", ex)
	}
	args = append(args, q, os.Getenv("HOME"))

	out, err := exec.Command("fd", args...).Output()
	if err != nil {
		return nil
	}

	paths := strings.Split(strings.TrimRight(string(out), "\n"), "\n")
	// shorter paths first: ~/notes.md beats a deep vendored hit
	sort.SliceStable(paths, func(i, j int) bool { return len(paths[i]) < len(paths[j]) })

	var acts []action.Action
	for _, path := range paths {
		if path == "" {
			continue
		}
		acts = append(acts, fileAction(path))
		if len(acts) == 25 {
			break
		}
	}
	return acts
}

func fileAction(path string) action.Action {
	display := path
	if home := os.Getenv("HOME"); strings.HasPrefix(path, home) {
		display = "~" + strings.TrimPrefix(path, home)
	}
	return action.Action{
		ID:    "file:" + path,
		Name:  display,
		Icon:  "󰈔",
		Group: "file",
		Cmd:   "uwsm-app -- xdg-open " + shell.Quote(path),
	}
}

func recentFiles(n int) []action.Action {
	data, err := os.ReadFile(filepath.Join(os.Getenv("HOME"), ".local/share/recently-used.xbel"))
	if err != nil {
		return nil
	}

	var xbel struct {
		Bookmarks []struct {
			Href     string `xml:"href,attr"`
			Modified string `xml:"modified,attr"`
		} `xml:"bookmark"`
	}
	if xml.Unmarshal(data, &xbel) != nil {
		return nil
	}

	sort.Slice(xbel.Bookmarks, func(i, j int) bool {
		return xbel.Bookmarks[i].Modified > xbel.Bookmarks[j].Modified
	})

	var acts []action.Action
	for _, b := range xbel.Bookmarks {
		path, ok := strings.CutPrefix(b.Href, "file://")
		if !ok {
			continue
		}
		if decoded, err := url.PathUnescape(path); err == nil {
			path = decoded
		}
		if _, err := os.Stat(path); err != nil {
			continue
		}
		acts = append(acts, fileAction(path))
		if len(acts) == n {
			break
		}
	}
	return acts
}
