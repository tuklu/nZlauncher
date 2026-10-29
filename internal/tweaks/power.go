// Package tweaks reads and changes system settings through whichever
// service the platform found for each: power profiles, do not disturb,
// nightlight, keeping the screen awake.
package tweaks

import (
	"errors"
	"fmt"

	"github.com/godbus/dbus/v5"
	"github.com/tuklu/nZlauncher/internal/platform"
)

// The power-profiles API, served by power-profiles-daemon, tlp-pd and
// tuned-ppd alike. Switching needs no password for the active session.
const (
	powerBus   = "org.freedesktop.UPower.PowerProfiles"
	powerPath  = "/org/freedesktop/UPower/PowerProfiles"
	powerIface = "org.freedesktop.UPower.PowerProfiles"
)

type PowerState struct {
	Daemon   string
	Active   string   // power-saver, balanced, performance
	Profiles []string // what this machine offers
}

var ErrNoPower = errors.New("no power profile service (power-profiles-daemon, tlp-pd or tuned-ppd)")

// Power reads the current profile; ok is false when no service serves the API.
func Power() (PowerState, bool) {
	daemon := platform.Current().Power
	if daemon == "" {
		return PowerState{}, false
	}
	conn, err := dbus.SystemBus()
	if err != nil {
		return PowerState{}, false
	}
	var props map[string]dbus.Variant
	if conn.Object(powerBus, powerPath).Call("org.freedesktop.DBus.Properties.GetAll", 0, powerIface).Store(&props) != nil {
		return PowerState{}, false
	}
	s := PowerState{Daemon: daemon}
	s.Active, _ = props["ActiveProfile"].Value().(string)
	profiles, _ := props["Profiles"].Value().([]map[string]dbus.Variant)
	for _, p := range profiles {
		if name, ok := p["Profile"].Value().(string); ok {
			s.Profiles = append(s.Profiles, name)
		}
	}
	return s, true
}

func SetPower(profile string) error {
	if platform.Current().Power == "" {
		return ErrNoPower
	}
	conn, err := dbus.SystemBus()
	if err != nil {
		return err
	}
	state, ok := Power()
	if !ok {
		return ErrNoPower
	}
	for _, p := range state.Profiles {
		if p == profile {
			return conn.Object(powerBus, powerPath).
				Call("org.freedesktop.DBus.Properties.Set", 0, powerIface, "ActiveProfile", dbus.MakeVariant(profile)).Err
		}
	}
	return fmt.Errorf("this machine has no %s profile", profile)
}
