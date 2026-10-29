package wifi

import (
	"fmt"
	"strings"
	"sync"

	"github.com/godbus/dbus/v5"
)

const (
	nmBus      = "org.freedesktop.NetworkManager"
	nmPath     = "/org/freedesktop/NetworkManager"
	nmSettings = "/org/freedesktop/NetworkManager/Settings"
	nmWifiType = 2 // NM_DEVICE_TYPE_WIFI
	// NM_802_11_AP_FLAGS_PRIVACY, and the key management bit for 802.1X
	apPrivacy    = 0x1
	keyMgmt8021x = 0x200
)

var networkManager = backend{
	bus:        nmBus,
	fetch:      nmFetch,
	connect:    nmConnect,
	disconnect: nmDisconnect,
	scan: func(conn *dbus.Conn) {
		if dev, _, err := nmWifiDevice(conn); err == nil {
			conn.Object(nmBus, dev).Call("org.freedesktop.NetworkManager.Device.Wireless.RequestScan", 0, map[string]dbus.Variant{})
		}
	},
}

// Saved profiles change rarely and cost one call each to read; access point
// strengths change all the time. So the known SSIDs are cached on their own
// and reread only after a signal from the settings objects.
var nmKnown struct {
	sync.Mutex
	stale    bool
	loaded   bool
	profiles map[string]dbus.ObjectPath // ssid -> connection profile
}

// settingsChanged is called for every signal; one about a profile marks the
// known list stale.
func settingsChanged(sig *dbus.Signal) {
	if strings.HasPrefix(string(sig.Path), nmSettings) {
		nmKnown.Lock()
		nmKnown.stale = true
		nmKnown.Unlock()
	}
}

func getAll(conn *dbus.Conn, path dbus.ObjectPath, iface string) (map[string]dbus.Variant, error) {
	var props map[string]dbus.Variant
	err := conn.Object(nmBus, path).Call("org.freedesktop.DBus.Properties.GetAll", 0, iface).Store(&props)
	return props, err
}

// nmWifiDevice is the first Wi-Fi device and its interface name.
func nmWifiDevice(conn *dbus.Conn) (dbus.ObjectPath, string, error) {
	var devices []dbus.ObjectPath
	if err := conn.Object(nmBus, nmPath).Call("org.freedesktop.NetworkManager.GetDevices", 0).Store(&devices); err != nil {
		return "", "", err
	}
	for _, d := range devices {
		props, err := getAll(conn, d, "org.freedesktop.NetworkManager.Device")
		if err != nil {
			continue
		}
		if t, _ := props["DeviceType"].Value().(uint32); t == nmWifiType {
			return d, variantString(props["Interface"]), nil
		}
	}
	return "", "", fmt.Errorf("NetworkManager has no Wi-Fi device")
}

func nmProfiles(conn *dbus.Conn) map[string]dbus.ObjectPath {
	nmKnown.Lock()
	defer nmKnown.Unlock()
	if nmKnown.loaded && !nmKnown.stale {
		return nmKnown.profiles
	}
	var paths []dbus.ObjectPath
	if conn.Object(nmBus, nmSettings).Call("org.freedesktop.NetworkManager.Settings.ListConnections", 0).Store(&paths) != nil {
		return nmKnown.profiles
	}
	profiles := map[string]dbus.ObjectPath{}
	for _, p := range paths {
		var settings map[string]map[string]dbus.Variant
		if conn.Object(nmBus, p).Call("org.freedesktop.NetworkManager.Settings.Connection.GetSettings", 0).Store(&settings) != nil {
			continue
		}
		if ssid, ok := settings["802-11-wireless"]["ssid"].Value().([]byte); ok {
			profiles[string(ssid)] = p
		}
	}
	nmKnown.profiles, nmKnown.loaded, nmKnown.stale = profiles, true, false
	return profiles
}

func nmFetch(conn *dbus.Conn) (State, error) {
	dev, iface, err := nmWifiDevice(conn)
	if err != nil {
		return State{}, err
	}
	wireless, err := getAll(conn, dev, "org.freedesktop.NetworkManager.Device.Wireless")
	if err != nil {
		return State{}, err
	}
	active, _ := wireless["ActiveAccessPoint"].Value().(dbus.ObjectPath)
	aps, _ := wireless["AccessPoints"].Value().([]dbus.ObjectPath)
	profiles := nmProfiles(conn)

	var points []accessPoint
	for _, ap := range aps {
		props, err := getAll(conn, ap, "org.freedesktop.NetworkManager.AccessPoint")
		if err != nil {
			continue
		}
		ssid, _ := props["Ssid"].Value().([]byte)
		strength, _ := props["Strength"].Value().(byte)
		flags, _ := props["Flags"].Value().(uint32)
		wpa, _ := props["WpaFlags"].Value().(uint32)
		rsn, _ := props["RsnFlags"].Value().(uint32)
		points = append(points, accessPoint{string(ssid), int(strength), flags, wpa, rsn, ap == active})
	}
	return State{Interface: iface, Networks: dedupe(nmNetworks(points, profiles))}, nil
}

type accessPoint struct {
	ssid            string
	strength        int
	flags, wpa, rsn uint32
	active          bool
}

func nmNetworks(points []accessPoint, profiles map[string]dbus.ObjectPath) []Network {
	var ns []Network
	for _, p := range points {
		security := "open"
		switch {
		case (p.wpa|p.rsn)&keyMgmt8021x != 0:
			security = "8021x"
		case p.wpa|p.rsn != 0:
			security = "psk"
		case p.flags&apPrivacy != 0:
			security = "wep"
		}
		_, known := profiles[p.ssid]
		ns = append(ns, Network{SSID: p.ssid, Security: security, Signal: p.strength, Known: known, Connected: p.active})
	}
	return ns
}

func nmConnect(conn *dbus.Conn, ssid string) error {
	dev, _, err := nmWifiDevice(conn)
	if err != nil {
		return err
	}
	nm := conn.Object(nmBus, nmPath)
	if profile, ok := nmProfiles(conn)[ssid]; ok {
		return nm.Call("org.freedesktop.NetworkManager.ActivateConnection", 0, profile, dev, dbus.ObjectPath("/")).Err
	}
	// an open network needs no profile: NM makes one from the access point
	ap, security, ok := nmAccessPoint(conn, dev, ssid)
	if !ok {
		return fmt.Errorf("%s is not in range", ssid)
	}
	if security != "open" {
		return fmt.Errorf("%s needs a passphrase", ssid)
	}
	// NM fills in the rest; naming the profile after the network keeps it
	// readable in nmcli and settings
	settings := map[string]map[string]dbus.Variant{
		"connection":      {"id": dbus.MakeVariant(ssid), "type": dbus.MakeVariant("802-11-wireless")},
		"802-11-wireless": {"ssid": dbus.MakeVariant([]byte(ssid))},
	}
	return nm.Call("org.freedesktop.NetworkManager.AddAndActivateConnection", 0, settings, dev, ap).Err
}

// nmAccessPoint is the strongest access point of a network and its security.
func nmAccessPoint(conn *dbus.Conn, dev dbus.ObjectPath, ssid string) (dbus.ObjectPath, string, bool) {
	wireless, err := getAll(conn, dev, "org.freedesktop.NetworkManager.Device.Wireless")
	if err != nil {
		return "", "", false
	}
	aps, _ := wireless["AccessPoints"].Value().([]dbus.ObjectPath)
	var best dbus.ObjectPath
	strongest, security := -1, ""
	for _, ap := range aps {
		props, err := getAll(conn, ap, "org.freedesktop.NetworkManager.AccessPoint")
		if err != nil {
			continue
		}
		if name, _ := props["Ssid"].Value().([]byte); string(name) != ssid {
			continue
		}
		strength, _ := props["Strength"].Value().(byte)
		if int(strength) > strongest {
			flags, _ := props["Flags"].Value().(uint32)
			wpa, _ := props["WpaFlags"].Value().(uint32)
			rsn, _ := props["RsnFlags"].Value().(uint32)
			n := nmNetworks([]accessPoint{{ssid, int(strength), flags, wpa, rsn, false}}, nil)[0]
			best, strongest, security = ap, int(strength), n.Security
		}
	}
	return best, security, best != ""
}

func nmDisconnect(conn *dbus.Conn) error {
	dev, _, err := nmWifiDevice(conn)
	if err != nil {
		return err
	}
	return conn.Object(nmBus, dev).Call("org.freedesktop.NetworkManager.Device.Disconnect", 0).Err
}
