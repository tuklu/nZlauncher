package providers

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/tuklu/nZlauncher/internal/action"
	"github.com/tuklu/nZlauncher/internal/shell"
)

const clipLimit = 30

type clipEntry struct {
	id, preview string
	image       *clipImage
}

type clipImage struct {
	size, format string // "2 MiB", "png"
	w, h         string
}

// cliphist describes an image as "[[ binary data 2 MiB png 1928x1886 ]]".
var binaryData = regexp.MustCompile(`^\[\[ binary data (\S+ \S+) (\w+) (\d+)x(\d+) \]\]$`)

// cliphist list reads its whole database (150ms on a large one), so the
// parsed list is kept until the database file changes.
var clips struct {
	sync.Mutex
	stamp   string
	entries []clipEntry
}

func clipDB() string {
	if dir := os.Getenv("XDG_CACHE_HOME"); dir != "" {
		return filepath.Join(dir, "cliphist", "db")
	}
	return filepath.Join(os.Getenv("HOME"), ".cache", "cliphist", "db")
}

func clipEntries() []clipEntry {
	clips.Lock()
	defer clips.Unlock()
	stamp := ""
	if info, err := os.Stat(clipDB()); err == nil {
		stamp = fmt.Sprint(info.ModTime().UnixNano(), info.Size())
	}
	if stamp != "" && stamp == clips.stamp {
		return clips.entries
	}
	out, err := exec.Command("cliphist", "list").Output()
	if err != nil {
		return nil
	}
	var entries []clipEntry
	for line := range bytes.SplitSeq(bytes.TrimRight(out, "\n"), []byte{'\n'}) {
		id, preview, ok := strings.Cut(string(line), "\t")
		if !ok {
			continue
		}
		e := clipEntry{id: id, preview: strings.TrimSpace(preview)}
		if m := binaryData.FindStringSubmatch(e.preview); m != nil {
			e.image = &clipImage{size: m[1], format: m[2], w: m[3], h: m[4]}
		}
		entries = append(entries, e)
	}
	clips.stamp, clips.entries = stamp, entries
	return entries
}

// Image entries are decoded into the cache by one background worker; a
// result only names a preview once its file exists, so no keystroke waits.
var decodeQueue = make(chan clipEntry, 256)
var startDecoder sync.Once

func clipImageFile(e clipEntry) string {
	return filepath.Join(cacheDir(), "clipboard", e.id+"."+e.image.format)
}

func clipImagePath(e clipEntry) string {
	path := clipImageFile(e)
	if _, err := os.Stat(path); err == nil {
		return path
	}
	startDecoder.Do(func() { go decodeImages() })
	select {
	case decodeQueue <- e:
	default: // queue full: a later query asks again
	}
	return ""
}

func decodeImages() {
	for e := range decodeQueue {
		path := clipImageFile(e)
		if _, err := os.Stat(path); err == nil || os.MkdirAll(filepath.Dir(path), 0o700) != nil {
			continue
		}
		cmd := exec.Command("cliphist", "decode")
		cmd.Stdin = strings.NewReader(e.id)
		if data, err := cmd.Output(); err == nil {
			// write then rename, so a reader never sees half an image
			tmp := path + ".part"
			if os.WriteFile(tmp, data, 0o600) == nil {
				os.Rename(tmp, path)
			}
		}
	}
}

// WarmClipboard queues the newest images for decoding; the daemon calls it.
func WarmClipboard() {
	n := 0
	for _, e := range clipEntries() {
		if e.image != nil && n < clipLimit {
			clipImagePath(e)
			n++
		}
	}
}

func cacheDir() string {
	if dir := os.Getenv("XDG_CACHE_HOME"); dir != "" {
		return filepath.Join(dir, "nzlauncher")
	}
	return filepath.Join(os.Getenv("HOME"), ".cache", "nzlauncher")
}

// clip / clipboard: history via cliphist; picking copies the entry again.
// "clip image" lists only images.
func Clipboard(q string) []action.Action {
	if !On("clipboard") {
		return nil
	}
	q = strings.ToLower(strings.TrimSpace(q))
	onlyImages := q == "image" || q == "images" || q == "img"

	var acts []action.Action
	for _, e := range clipEntries() {
		name, hint, icon, preview := e.preview, "", "󰅍", ""
		if e.image != nil {
			name = fmt.Sprintf("Image %s×%s", e.image.w, e.image.h)
			hint = e.image.format + " · " + e.image.size
			icon = "󰋩"
		}
		switch {
		case onlyImages && e.image == nil:
			continue
		case !onlyImages && q != "" && !strings.Contains(strings.ToLower(name+" "+hint), q):
			continue
		}
		if e.image != nil {
			preview = clipImagePath(e)
		}
		display := []rune(name)
		if len(display) > 70 {
			display = append(display[:67], []rune("...")...)
		}
		acts = append(acts, action.Action{
			ID: "clip", Name: string(display), Icon: icon, Group: "clipboard",
			Hint: hint, Preview: preview,
			Cmd: "printf %s " + shell.Quote(e.id) + " | cliphist decode | wl-copy",
		})
		if len(acts) == clipLimit {
			break
		}
	}
	return acts
}

// PruneClipCache drops decoded images older than a week; the daemon calls it.
func PruneClipCache() {
	dir := filepath.Join(cacheDir(), "clipboard")
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if info, err := e.Info(); err == nil && time.Since(info.ModTime()) > 7*24*time.Hour {
			os.Remove(filepath.Join(dir, e.Name()))
		}
	}
}
