// Package identity holds the names nZlauncher shows the desktop. Window and layer
// rules match on them, so they only change with a note in docs/hyprland.md.
package identity

const (
	// GTK application id, reverse DNS of a domain the author controls; it is
	// also the GTK face's window class.
	AppID = "dev.tuklu.nzlauncher"
	// Window classes of the terminal windows nZlauncher opens.
	TUIClass  = AppID + ".tui"
	PickClass = AppID + ".pick"
	// Layer-shell namespace of the GTK face in overlay mode.
	Namespace = "nzlauncher"
)
