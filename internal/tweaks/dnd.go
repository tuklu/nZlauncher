package tweaks

import (
	"errors"
	"slices"
	"strings"

	"github.com/godbus/dbus/v5"
	"github.com/tuklu/nZlauncher/internal/platform"
)

// Do not disturb, through the notification daemon's own interface: mako
// modes, swaync's dnd switch, dunst's pause.

// makoMode is the mode mako configs conventionally call do not disturb.
const makoMode = "do-not-disturb"

type dndBackend struct {
	get func(conn *dbus.Conn) (bool, error)
	set func(conn *dbus.Conn, on bool) error
}

var dndBackends = map[string]dndBackend{
	"mako": {
		get: func(conn *dbus.Conn) (bool, error) {
			modes, err := makoModes(conn)
			return slices.Contains(modes, makoMode), err
		},
		set: func(conn *dbus.Conn, on bool) error {
			modes, err := makoModes(conn)
			if err != nil {
				return err
			}
			modes = slices.DeleteFunc(modes, func(m string) bool { return m == makoMode })
			if on {
				modes = append(modes, makoMode)
			}
			if len(modes) == 0 {
				modes = []string{"default"}
			}
			return conn.Object("org.freedesktop.Notifications", "/fr/emersion/Mako").Call("fr.emersion.Mako.SetModes", 0, modes).Err
		},
	},
	"swaync": {
		get: func(conn *dbus.Conn) (bool, error) {
			var on bool
			err := swaync(conn).Call("org.erikreider.swaync.cc.GetDnd", 0).Store(&on)
			return on, err
		},
		set: func(conn *dbus.Conn, on bool) error {
			return swaync(conn).Call("org.erikreider.swaync.cc.SetDnd", 0, on).Err
		},
	},
	"dunst": {
		get: func(conn *dbus.Conn) (bool, error) {
			v, err := dunst(conn).GetProperty("org.dunstproject.cmd0.paused")
			on, _ := v.Value().(bool)
			return on, err
		},
		set: func(conn *dbus.Conn, on bool) error {
			return dunst(conn).SetProperty("org.dunstproject.cmd0.paused", dbus.MakeVariant(on))
		},
	},
}

func makoModes(conn *dbus.Conn) ([]string, error) {
	v, err := conn.Object("org.freedesktop.Notifications", "/fr/emersion/Mako").GetProperty("fr.emersion.Mako.Modes")
	modes, _ := v.Value().([]string)
	return modes, err
}

func swaync(conn *dbus.Conn) dbus.BusObject {
	return conn.Object("org.erikreider.swaync.cc", "/org/erikreider/swaync/cc")
}

func dunst(conn *dbus.Conn) dbus.BusObject {
	return conn.Object("org.freedesktop.Notifications", "/org/freedesktop/Notifications")
}

// dndFor maps the name a daemon reports ("SwayNotificationCenter") to its backend.
func dndFor(server string) (dndBackend, string, bool) {
	for name, b := range dndBackends {
		if strings.Contains(server, name) || name == "swaync" && strings.Contains(server, "swaynotification") {
			return b, name, true
		}
	}
	return dndBackend{}, "", false
}

var ErrNoDND = errors.New("the notification daemon has no do not disturb nzlauncher knows (mako, swaync, dunst)")

// DND reports whether do not disturb is on; ok is false when the daemon is
// not one of the supported ones.
func DND() (on bool, daemon string, ok bool) {
	b, name, found := dndFor(platform.Current().Notifications)
	if !found {
		return false, "", false
	}
	conn, err := dbus.SessionBus()
	if err != nil {
		return false, name, false
	}
	on, err = b.get(conn)
	return on, name, err == nil
}

func SetDND(on bool) error {
	b, _, found := dndFor(platform.Current().Notifications)
	if !found {
		return ErrNoDND
	}
	conn, err := dbus.SessionBus()
	if err != nil {
		return err
	}
	return b.set(conn, on)
}
