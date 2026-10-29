// Package compositor is what the launcher asks of the window manager: the
// open windows, focusing one, workspaces, and word that windows changed.
// Hyprland, sway and niri each answer over their own socket; the platform
// says which one runs.
package compositor

import (
	"errors"

	"github.com/tuklu/nZlauncher/internal/platform"
)

// Window is an open window. ID is the compositor's own handle for it: a
// Hyprland address, a sway container id, a niri window id.
type Window struct {
	ID    string
	Class string // app id or X11 class
	Title string
	PID   int
	// Workspace is where the window is, as the compositor names it
	Workspace string
}

type backend struct {
	windows   func() ([]Window, error)
	focus     func(id string) error
	workspace func(n int) error
	moveTo    func(n int) error
	focused   func() (Window, error)
	// watch calls changed whenever the window list may differ, until the
	// connection ends
	watch func(changed func()) error
	// onScreen reports whether any window or layer surface mentions one
	// of the markers
	onScreen func(markers []string) bool
}

var backends = map[string]backend{
	"hyprland": hyprland,
	"sway":     sway,
	"niri":     niri,
}

// ErrUnsupported: the compositor is none nzlauncher knows.
var ErrUnsupported = errors.New("window actions need Hyprland, sway or niri")

func current() (backend, error) {
	b, ok := backends[platform.Current().Compositor]
	if !ok {
		return backend{}, ErrUnsupported
	}
	return b, nil
}

// Supported reports whether window actions work here.
func Supported() bool {
	_, err := current()
	return err == nil
}

func Windows() ([]Window, error) {
	b, err := current()
	if err != nil {
		return nil, err
	}
	return b.windows()
}

func Focus(id string) error {
	b, err := current()
	if err != nil {
		return err
	}
	return b.focus(id)
}

// Workspace switches to workspace n.
func Workspace(n int) error {
	b, err := current()
	if err != nil {
		return err
	}
	return b.workspace(n)
}

// MoveToWorkspace sends the focused window to workspace n.
func MoveToWorkspace(n int) error {
	b, err := current()
	if err != nil {
		return err
	}
	return b.moveTo(n)
}

// Focused is the window that has focus.
func Focused() (Window, error) {
	b, err := current()
	if err != nil {
		return Window{}, err
	}
	return b.focused()
}

// Watch calls changed whenever the window list may have changed, until the
// compositor's event stream ends. It calls it once at the start, for what
// happened before it was listening.
func Watch(changed func()) error {
	b, err := current()
	if err != nil {
		return err
	}
	return b.watch(changed)
}

// OnScreen reports whether a window or layer surface carries one of the
// markers (a class or a layer namespace).
func OnScreen(markers []string) bool {
	b, err := current()
	if err != nil {
		return false
	}
	return b.onScreen(markers)
}
