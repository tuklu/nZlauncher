package main

import (
	"os"
	"path/filepath"

	layershell "github.com/diamondburned/gotk4-layer-shell/pkg/gtk4layershell"
	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/gio/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

// The GTK face: layer-shell overlay window, real app icons, themed via
// ~/.config/neZha/theme/nz-gtk.css.
func runGTK(initial string) error {
	app := gtk.NewApplication("dev.tuklu.nz", gio.ApplicationFlagsNone)
	app.ConnectActivate(func() { buildGTKWindow(app, initial) })
	if code := app.Run(nil); code > 0 {
		os.Exit(code)
	}
	return nil
}

func gtkCSSPath() string {
	return filepath.Join(os.Getenv("HOME"), ".config/neZha/theme/nz-gtk.css")
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

func buildGTKWindow(app *gtk.Application, initial string) {
	win := gtk.NewApplicationWindow(app)
	win.AddCSSClass("nz")
	win.SetDefaultSize(760, 500)

	if gtkMode() == "overlay" && layershell.IsSupported() {
		w := &win.Window
		layershell.InitForWindow(w)
		layershell.SetNamespace(w, "nz-bar")
		layershell.SetLayer(w, layershell.LayerShellLayerOverlay)
		layershell.SetKeyboardMode(w, layershell.LayerShellKeyboardModeExclusive)
	}

	if _, err := os.Stat(gtkCSSPath()); err != nil {
		os.MkdirAll(filepath.Dir(gtkCSSPath()), 0o755)
		os.WriteFile(gtkCSSPath(), []byte(defaultGTKCSS), 0o644)
	}
	css := gtk.NewCSSProvider()
	css.LoadFromPath(gtkCSSPath())
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

	var actions []Action
	var rows []*gtk.ListBoxRow

	execute := func(idx int) {
		if idx >= 0 && idx < len(actions) {
			a := actions[idx]
			recordLaunch(a.ID, entry.Text())
			launch(a.Cmd)
		}
		win.Close()
	}

	refresh := func() {
		for _, r := range rows {
			list.Remove(r)
		}
		rows = rows[:0]

		acts, err := queryActions(entry.Text())
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
	keys.ConnectKeyPressed(func(keyval, _ uint, _ gdk.ModifierType) bool {
		switch keyval {
		case gdk.KEY_Escape:
			win.Close()
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

	entry.SetText(initial)
	entry.SetPosition(-1)
	refresh()
	entry.GrabFocus()
	win.SetVisible(true)
}
