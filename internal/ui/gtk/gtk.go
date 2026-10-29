// Package gtk is the GTK4 face: layer-shell overlay or managed window,
// real app icons, themed via ~/.config/nzlauncher/theme/nz-gtk.css.
package gtk

import (
	"encoding/json"
	"fmt"
	"os"

	layershell "github.com/diamondburned/gotk4-layer-shell/pkg/gtk4layershell"
	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/gio/v2"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"

	"github.com/tuklu/nZlauncher/internal/action"
	"github.com/tuklu/nZlauncher/internal/config"
	"github.com/tuklu/nZlauncher/internal/engine"
	"github.com/tuklu/nZlauncher/internal/identity"
	"github.com/tuklu/nZlauncher/internal/learn"
	"github.com/tuklu/nZlauncher/internal/server"
	"github.com/tuklu/nZlauncher/internal/shell"
)

// Run opens the face. A resident face stays alive hidden after a pick or
// Escape, and the bar shows it again through server.ToggleFace.
func Run(initial, mode string, resident bool) error {
	// no D-Bus name: the face socket already keeps one face per session, and
	// registering one only costs startup time
	app := gtk.NewApplication(identity.AppID, gio.ApplicationNonUnique)
	var picked string // one-shot face: command to run once the window is gone
	app.ConnectActivate(func() {
		f := buildWindow(app, mode, resident, &picked)
		f.show(initial, os.Getenv("NZ_FOCUSED"))
		if resident {
			app.Hold()
			go serveFace(f)
		}
	})
	if code := app.Run(nil); code > 0 {
		os.Exit(code)
	}
	if picked != "" {
		return shell.LaunchAfterClose(picked)
	}
	return nil
}

func gtkCSSPath() string {
	return config.ThemeFile("nz-gtk.css")
}

const defaultGTKCSS = `/* nZlauncher GTK theme - edit freely */
window.nz {
  background: rgba(24, 24, 32, 0.94);
  border-radius: 16px;
}
.nz-entry {
  font-size: 17px;
  margin: 14px 14px 6px 14px;
  border-radius: 10px;
  padding: 6px 10px;
}
.nz-list {
  background: transparent;
  margin: 0 8px 10px 8px;
}
.nz-list row {
  padding: 7px 12px;
  border-radius: 10px;
  margin: 1px 4px;
}
.nz-list row:selected {
  background: alpha(@accent_bg_color, 0.85);
}
.nz-name {
  font-size: 14px;
}
.nz-badge {
  font-size: 11px;
  opacity: 0.55;
}
`

// face is the one window, reused for every show in resident mode.
type face struct {
	win     *gtk.ApplicationWindow
	entry   *gtk.SearchEntry
	refresh func()
	focused string // class of the window the user came from
}

func (f *face) show(q, focused string) {
	f.focused = focused
	f.entry.SetText(q)
	f.entry.SetPosition(-1)
	f.refresh()
	f.win.SetVisible(true)
	f.win.Present()
	f.entry.GrabFocus()
}

func (f *face) toggle(q, focused string) {
	if f.win.IsVisible() {
		f.win.SetVisible(false)
		return
	}
	f.show(q, focused)
}

// serveFace answers the bar's toggle requests; GTK is only touched from its
// own thread, so each request is handed over with IdleAdd.
func serveFace(f *face) {
	ln, err := server.ListenFace()
	if err != nil {
		fmt.Fprintln(os.Stderr, "nzlauncher-gtk:", err)
		return
	}
	for {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		go func() {
			defer conn.Close()
			var req server.Request
			if err := json.NewDecoder(conn).Decode(&req); err != nil || req.Op != "toggle" {
				json.NewEncoder(conn).Encode(server.Response{Error: "want {\"op\":\"toggle\"}"})
				return
			}
			glib.IdleAdd(func() { f.toggle(req.Q, req.Focused) })
			json.NewEncoder(conn).Encode(server.Response{OK: true})
		}()
	}
}

func buildWindow(app *gtk.Application, mode string, resident bool, picked *string) *face {
	win := gtk.NewApplicationWindow(app)
	win.AddCSSClass("nz")
	win.SetDefaultSize(760, 500)

	if mode == "overlay" && layershell.IsSupported() {
		w := &win.Window
		layershell.InitForWindow(w)
		layershell.SetNamespace(w, identity.Namespace)
		layershell.SetLayer(w, layershell.LayerShellLayerOverlay)
		layershell.SetKeyboardMode(w, layershell.LayerShellKeyboardModeExclusive)
	}

	// built-in look unless a theme provides one; nothing is written
	css := gtk.NewCSSProvider()
	if _, err := os.Stat(gtkCSSPath()); err == nil {
		css.LoadFromPath(gtkCSSPath())
	} else {
		css.LoadFromString(defaultGTKCSS)
	}
	gtk.StyleContextAddProviderForDisplay(gdk.DisplayGetDefault(), css, 800)

	entry := gtk.NewSearchEntry()
	entry.AddCSSClass("nz-entry")
	entry.SetObjectProperty("placeholder-text", "Search anything…")

	list := gtk.NewListBox()
	list.AddCSSClass("nz-list")
	list.SetSelectionMode(gtk.SelectionSingle)

	scroll := gtk.NewScrolledWindow()
	scroll.SetVExpand(true)
	scroll.SetPolicy(gtk.PolicyNever, gtk.PolicyAutomatic)
	scroll.SetChild(list)

	box := gtk.NewBox(gtk.OrientationVertical, 0)
	box.Append(entry)
	box.Append(scroll)
	win.SetChild(box)

	var actions []action.Action
	var rows []*gtk.ListBoxRow
	f := &face{win: win, entry: entry}

	// resident: hide instead of closing, so the next show is instant
	dismiss := func() {
		if resident {
			win.SetVisible(false)
			return
		}
		win.Close()
	}
	if resident {
		win.ConnectCloseRequest(func() bool {
			win.SetVisible(false)
			return true
		})
	}

	execute := func(idx int) {
		if idx >= 0 && idx < len(actions) {
			a := actions[idx]
			learn.RecordFor(a.ID, entry.Text(), f.focused)
			if resident {
				win.SetVisible(false)
				shell.LaunchAfterHide(a.Cmd)
				return
			}
			*picked = a.Cmd
		}
		dismiss()
	}

	refresh := func() {
		for _, r := range rows {
			list.Remove(r)
		}
		rows = rows[:0]

		acts, err := engine.QueryFor(entry.Text(), f.focused)
		if err != nil {
			acts = nil
		}
		actions = acts

		for _, a := range actions {
			h := gtk.NewBox(gtk.OrientationHorizontal, 10)

			if a.IconName != "" {
				img := gtk.NewImageFromIconName(a.IconName)
				img.SetPixelSize(22)
				h.Append(img)
			} else {
				glyph := gtk.NewLabel(a.Icon)
				glyph.SetWidthChars(2)
				h.Append(glyph)
			}

			name := gtk.NewLabel(a.Name)
			name.AddCSSClass("nz-name")
			name.SetXAlign(0)
			name.SetHExpand(true)
			name.SetEllipsize(3) // PANGO_ELLIPSIZE_END
			h.Append(name)

			badge := gtk.NewLabel(a.Group)
			badge.AddCSSClass("nz-badge")
			h.Append(badge)

			row := gtk.NewListBoxRow()
			row.SetChild(h)
			list.Append(row)
			rows = append(rows, row)
		}
		if len(rows) > 0 {
			list.SelectRow(rows[0])
		}
	}

	move := func(delta int) {
		sel := list.SelectedRow()
		idx := 0
		if sel != nil {
			idx = sel.Index() + delta
		}
		if idx >= 0 && idx < len(rows) {
			list.SelectRow(rows[idx])
			rows[idx].GrabFocus()
			entry.GrabFocus()
		}
	}

	entry.ConnectSearchChanged(refresh)
	entry.ConnectActivate(func() {
		idx := 0
		if sel := list.SelectedRow(); sel != nil {
			idx = sel.Index()
		}
		execute(idx)
	})
	list.ConnectRowActivated(func(row *gtk.ListBoxRow) {
		execute(row.Index())
	})

	keys := gtk.NewEventControllerKey()
	// capture phase: the SearchEntry consumes Escape itself otherwise
	keys.SetPropagationPhase(gtk.PhaseCapture)
	keys.ConnectKeyPressed(func(keyval, _ uint, _ gdk.ModifierType) bool {
		switch keyval {
		case gdk.KEY_Escape:
			dismiss()
			return true
		case gdk.KEY_Down:
			move(1)
			return true
		case gdk.KEY_Up:
			move(-1)
			return true
		}
		return false
	})
	win.AddController(keys)

	f.refresh = refresh
	return f
}
