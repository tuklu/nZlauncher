package providers

import (
	"encoding/xml"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/tuklu/nZlauncher/internal/action"
	"github.com/tuklu/nZlauncher/internal/files"
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
	if hits, ok := files.Search(files.Filter{Terms: strings.Fields(strings.ToLower(q))}); ok {
		return FileActions(hits)
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
	return action.Action{
		ID:    "file:" + path,
		Name:  tilde(path),
		Icon:  fileIcon(path),
		Group: "file",
		Cmd:   "uwsm-app -- xdg-open " + shell.Quote(path),
	}
}

// FileActions turns index hits into results: the name first, where it is,
// how old and how big as the hint, and a preview for images and text.
func FileActions(hits []files.Hit) []action.Action {
	acts := make([]action.Action, 0, len(hits))
	now := time.Now()
	for _, h := range hits {
		a := fileAction(h.Path)
		a.Name = filepath.Base(h.Path)
		a.Hint = tilde(filepath.Dir(h.Path)) + " · " + age(now.Sub(h.ModTime)) + " · " + humanSize(h.Size)
		a.Keywords = []string{h.Path}
		if previewable(h.Path) {
			a.Preview = h.Path
		}
		acts = append(acts, a)
	}
	return acts
}

func tilde(path string) string {
	if home := os.Getenv("HOME"); path == home || strings.HasPrefix(path, home+"/") {
		return "~" + strings.TrimPrefix(path, home)
	}
	return path
}

func age(d time.Duration) string {
	switch {
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 48*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	case d < 60*24*time.Hour:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	case d < 2*365*24*time.Hour:
		return fmt.Sprintf("%dmo", int(d.Hours()/24/30))
	}
	return fmt.Sprintf("%dy", int(d.Hours()/24/365))
}

func humanSize(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for m := n / unit; m >= unit; m /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(n)/float64(div), "KMGTPE"[exp])
}

var imageExts = map[string]bool{"png": true, "jpg": true, "jpeg": true, "webp": true, "gif": true, "svg": true, "bmp": true}
var textExts = map[string]bool{"txt": true, "md": true, "go": true, "py": true, "c": true, "h": true, "rs": true, "lua": true,
	"sh": true, "toml": true, "json": true, "yaml": true, "yml": true, "conf": true, "js": true, "ts": true, "css": true, "html": true}

func extOf(path string) string {
	return strings.ToLower(strings.TrimPrefix(filepath.Ext(path), "."))
}

func previewable(path string) bool {
	ext := extOf(path)
	return imageExts[ext] || textExts[ext]
}

func fileIcon(path string) string {
	ext := extOf(path)
	switch {
	case imageExts[ext]:
		return "󰋩"
	case ext == "pdf":
		return "󰈦"
	case textExts[ext]:
		return "󰈙"
	case ext == "mp4" || ext == "mkv" || ext == "webm" || ext == "mov":
		return "󰈫"
	case ext == "mp3" || ext == "flac" || ext == "ogg" || ext == "wav" || ext == "m4a":
		return "󰈣"
	case ext == "zip" || ext == "tar" || ext == "gz" || ext == "xz" || ext == "zst" || ext == "7z":
		return "󰗄"
	}
	return "󰈔"
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
