package compositor

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/tuklu/nZlauncher/internal/hypr"
)

// hyprWindowEvents change what the window list shows.
var hyprWindowEvents = map[string]bool{
	"openwindow": true, "closewindow": true, "movewindow": true, "movewindowv2": true,
	"windowtitle": true, "windowtitlev2": true,
}

var hyprland = backend{
	windows: func() ([]Window, error) {
		out, err := hypr.JSON("clients")
		if err != nil {
			return nil, err
		}
		var clients []struct {
			Address string `json:"address"`
			Class   string `json:"class"`
			Title   string `json:"title"`
			Mapped  bool   `json:"mapped"`
		}
		if err := json.Unmarshal(out, &clients); err != nil {
			return nil, err
		}
		var ws []Window
		for _, c := range clients {
			if c.Mapped {
				ws = append(ws, Window{ID: c.Address, Class: c.Class, Title: c.Title})
			}
		}
		return ws, nil
	},
	focus: hypr.Focus,
	workspace: func(n int) error {
		return hypr.Dispatch(fmt.Sprintf("workspace %d", n), fmt.Sprintf("hl.dsp.focus({ workspace = %d })", n))
	},
	moveTo: func(n int) error {
		return hypr.Dispatch(fmt.Sprintf("movetoworkspace %d", n), fmt.Sprintf("hl.dsp.window.move({ workspace = %d })", n))
	},
	focused: func() (Window, error) {
		out, err := hypr.JSON("activewindow")
		if err != nil {
			return Window{}, err
		}
		var w struct {
			Address string `json:"address"`
			Class   string `json:"class"`
			Title   string `json:"title"`
		}
		err = json.Unmarshal(out, &w)
		return Window{ID: w.Address, Class: w.Class, Title: w.Title}, err
	},
	watch: func(changed func()) error {
		return hypr.Watch(func(e string) bool { return hyprWindowEvents[e] }, changed)
	},
	onScreen: func(markers []string) bool {
		for _, what := range []string{"clients", "layers"} {
			out, err := hypr.JSON(what)
			if err != nil {
				return false
			}
			for _, m := range markers {
				if bytes.Contains(out, []byte(m)) {
					return true
				}
			}
		}
		return false
	},
}
