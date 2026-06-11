package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

var pictureExts = map[string]bool{".jpg": true, ".jpeg": true, ".png": true, ".webp": true, ".heic": true, ".avif": true}
var videoExts = map[string]bool{".mp4": true, ".mov": true, ".m4v": true, ".mkv": true, ".webm": true, ".avi": true, ".gif": true}

// transcode / convert — two bar hops: pick a media file, then pick the
// format+resolution combo. The combo entry runs the transcode script
// non-interactively with all args.
func transcodeActions(q string) []Action {
	if path, ok := quotedPath(q); ok {
		return transcodeCombos(path)
	}

	out, err := exec.Command("bash", "-c",
		`find ~/Pictures ~/Videos ~/Downloads -maxdepth 3 -type f \( `+
			`-iname '*.jpg' -o -iname '*.jpeg' -o -iname '*.png' -o -iname '*.webp' `+
			`-o -iname '*.heic' -o -iname '*.avif' -o -iname '*.gif' -o -iname '*.mp4' `+
			`-o -iname '*.mov' -o -iname '*.m4v' -o -iname '*.mkv' -o -iname '*.webm' `+
			`-o -iname '*.avi' \) -printf '%T@\t%p\n' 2>/dev/null | sort -rn | cut -f2`).Output()
	if err != nil {
		return nil
	}

	home := os.Getenv("HOME")
	var acts []Action
	for _, path := range strings.Split(strings.TrimRight(string(out), "\n"), "\n") {
		if path == "" {
			continue
		}
		display := path
		if strings.HasPrefix(display, home) {
			display = "~" + strings.TrimPrefix(display, home)
		}
		if q != "" && !strings.Contains(strings.ToLower(display), strings.ToLower(q)) {
			continue
		}
		// reopen the bar at step 2; small sleep lets the closing window die
		// so runBar's toggle check doesn't see it as already open
		acts = append(acts, Action{
			ID: "tc:" + path, Name: "Transcode " + display, Icon: "󰈟", Group: "transcode",
			Cmd: "sleep 0.2 && nz bar " + shellQuote(`transcode "`+path+`" `),
		})
		if len(acts) == 25 {
			break
		}
	}
	return acts
}

func transcodeCombos(path string) []Action {
	base := filepath.Base(path)
	ext := strings.ToLower(filepath.Ext(path))

	type combo struct{ format, resolution string }
	var combos []combo
	switch {
	case pictureExts[ext]:
		for _, f := range []string{"jpg", "png"} {
			for _, r := range []string{"high", "medium", "low"} {
				combos = append(combos, combo{f, r})
			}
		}
	case videoExts[ext]:
		for _, f := range []string{"mp4", "gif"} {
			for _, r := range []string{"1080p", "720p", "4k"} {
				combos = append(combos, combo{f, r})
			}
		}
	default:
		return nil
	}

	var acts []Action
	for _, c := range combos {
		acts = append(acts, Action{
			ID:    "tc:" + c.format + ":" + c.resolution,
			Name:  base + "  →  " + c.format + " " + c.resolution,
			Icon:  "󰈟",
			Group: "transcode",
			Cmd:   "transcode " + shellQuote(path) + " " + c.format + " " + c.resolution,
		})
	}
	return acts
}

// quotedPath extracts `"...existing file..."` from the start of a query.
func quotedPath(q string) (string, bool) {
	q = strings.TrimSpace(q)
	if !strings.HasPrefix(q, `"`) {
		return "", false
	}
	end := strings.Index(q[1:], `"`)
	if end < 0 {
		return "", false
	}
	path := q[1 : end+1]
	if info, err := os.Stat(path); err != nil || info.IsDir() {
		return "", false
	}
	return path, true
}
