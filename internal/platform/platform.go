// Package platform finds out what the machine runs, by asking for running
// services instead of guessing from the distro: Fedora KDE is KWin plus
// NetworkManager plus power-profiles-daemon, and an Arch box can run any mix.
// Each capability gets one backend; config.toml's [backends] can name one.
package platform

import (
	"bufio"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/godbus/dbus/v5"
)

// Setup names the backend behind each capability; "" means none found.
type Setup struct {
	Compositor    string // hyprland, niri, sway, kde, gnome
	Wifi          string // iwd, networkmanager, wpa_supplicant
	Power         string // power-profiles-daemon, tuned
	Notifications string // mako, swaync, dunst, ... as the daemon names itself
	Distro        string // ID from os-release
	// config is read-only or rebuilt (NixOS, ostree), so a tweak has to be a
	// runtime change, never an edit of a system file
	Immutable bool
}

// Capabilities are the keys [backends] takes.
var Capabilities = []string{"compositor", "wifi", "power", "notifications"}

var (
	overrides atomic.Pointer[map[string]string]
	cached    struct {
		sync.Mutex
		at    time.Time
		setup Setup
	}
)

// Configure sets the backends named in config.toml.
func Configure(backends map[string]string) {
	overrides.Store(&backends)
}

// Services come and go (NetworkManager restarts, a notification daemon is
// swapped), so detection is redone at most every few seconds.
const redetectAfter = 5 * time.Second

// Current is the detected setup with config overrides applied.
func Current() Setup {
	cached.Lock()
	defer cached.Unlock()
	if time.Since(cached.at) > redetectAfter {
		cached.setup, cached.at = Detect(), time.Now()
	}
	s := cached.setup
	if o := overrides.Load(); o != nil {
		for k, v := range *o {
			switch k {
			case "compositor":
				s.Compositor = v
			case "wifi":
				s.Wifi = v
			case "power":
				s.Power = v
			case "notifications":
				s.Notifications = v
			}
		}
	}
	return s
}

// Detect looks at the machine now, without overrides.
func Detect() Setup {
	s := Setup{Compositor: compositor()}
	s.Distro, s.Immutable = distro()
	if names := busNames(dbus.SystemBus, "ListNames"); names != nil {
		// NetworkManager can drive iwd or wpa_supplicant underneath; it is
		// the one to talk to when it runs
		s.Wifi = first(names, map[string]string{
			"org.freedesktop.NetworkManager": "networkmanager",
			"net.connman.iwd":                "iwd",
			"fi.w1.wpa_supplicant1":          "wpa_supplicant",
		}, "org.freedesktop.NetworkManager", "net.connman.iwd", "fi.w1.wpa_supplicant1")
		// power-profiles-daemon starts on its first call: installed counts
		for n := range busNames(dbus.SystemBus, "ListActivatableNames") {
			names[n] = true
		}
		s.Power = first(names, map[string]string{
			"org.freedesktop.UPower.PowerProfiles": "power-profiles-daemon",
			"net.hadess.PowerProfiles":             "power-profiles-daemon",
			"com.redhat.tuned":                     "tuned",
		}, "org.freedesktop.UPower.PowerProfiles", "net.hadess.PowerProfiles", "com.redhat.tuned")
	}
	s.Notifications = notificationServer()
	return s
}

func first(names map[string]bool, backend map[string]string, order ...string) string {
	for _, n := range order {
		if names[n] {
			return backend[n]
		}
	}
	return ""
}

func compositor() string {
	switch {
	case os.Getenv("HYPRLAND_INSTANCE_SIGNATURE") != "":
		return "hyprland"
	case os.Getenv("NIRI_SOCKET") != "":
		return "niri"
	case os.Getenv("SWAYSOCK") != "":
		return "sway"
	}
	desktop := strings.ToLower(os.Getenv("XDG_CURRENT_DESKTOP"))
	for _, d := range []string{"hyprland", "niri", "sway", "kde", "gnome", "cosmic", "river", "wayfire", "labwc"} {
		if strings.Contains(desktop, d) {
			return d
		}
	}
	return ""
}

func distro() (id string, immutable bool) {
	if _, err := os.Stat("/run/ostree-booted"); err == nil {
		immutable = true
	}
	f, err := os.Open("/etc/os-release")
	if err != nil {
		return "", immutable
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if v, ok := strings.CutPrefix(sc.Text(), "ID="); ok {
			id = strings.Trim(v, `"`)
		}
	}
	return id, immutable || id == "nixos"
}

// busNames lists the bus names running (ListNames) or startable on demand
// (ListActivatableNames).
func busNames(connect func() (*dbus.Conn, error), method string) map[string]bool {
	conn, err := connect()
	if err != nil {
		return nil
	}
	var list []string
	if conn.BusObject().Call("org.freedesktop.DBus."+method, 0).Store(&list) != nil {
		return nil
	}
	names := make(map[string]bool, len(list))
	for _, n := range list {
		names[n] = true
	}
	return names
}

// notificationServer asks the running daemon its name: "mako", "swaync",
// "dunst", "Plasma", "gnome-shell".
func notificationServer() string {
	conn, err := dbus.SessionBus()
	if err != nil {
		return ""
	}
	var name, vendor, version, spec string
	call := conn.Object("org.freedesktop.Notifications", "/org/freedesktop/Notifications").
		Call("org.freedesktop.Notifications.GetServerInformation", dbus.FlagNoAutoStart)
	if call.Store(&name, &vendor, &version, &spec) != nil {
		return ""
	}
	return strings.ToLower(name)
}
