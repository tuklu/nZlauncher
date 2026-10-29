// Package bluez reads Bluetooth state from BlueZ over the system bus. One
// GetManagedObjects call answers everything, and BlueZ's change signals keep
// the cached copy current, so a query never waits on bluetoothctl.
package bluez

import (
	"sort"
	"strings"
	"sync"

	"github.com/godbus/dbus/v5"
)

type Device struct {
	Address   string
	Name      string
	Icon      string // freedesktop name BlueZ reports: input-mouse, audio-headset...
	Paired    bool
	Connected bool
	Battery   int // percent, -1 when the device does not report it
}

type State struct {
	Available bool // an adapter exists
	Powered   bool
	Devices   []Device
}

var cache struct {
	sync.Mutex
	conn  *dbus.Conn
	stale bool
	state State
}

// Current is the cached state, refetched only after BlueZ signalled a change.
func Current() State {
	cache.Lock()
	defer cache.Unlock()
	if cache.conn == nil {
		conn, err := dbus.ConnectSystemBus()
		if err != nil {
			return State{}
		}
		cache.conn, cache.stale = conn, true
		watch(conn)
	}
	if cache.stale {
		state, err := fetch(cache.conn)
		if err != nil {
			return State{}
		}
		cache.state, cache.stale = state, false
	}
	return cache.state
}

// Every BlueZ change arrives as PropertiesChanged or an object coming or
// going; any of them means the next query refetches.
func watch(conn *dbus.Conn) {
	conn.AddMatchSignal(dbus.WithMatchSender("org.bluez"), dbus.WithMatchInterface("org.freedesktop.DBus.Properties"))
	conn.AddMatchSignal(dbus.WithMatchSender("org.bluez"), dbus.WithMatchInterface("org.freedesktop.DBus.ObjectManager"))
	// BlueZ restarting is a new owner of the name
	conn.AddMatchSignal(dbus.WithMatchInterface("org.freedesktop.DBus"), dbus.WithMatchMember("NameOwnerChanged"), dbus.WithMatchArg(0, "org.bluez"))
	signals := make(chan *dbus.Signal, 64)
	conn.Signal(signals)
	go func() {
		for range signals {
			cache.Lock()
			cache.stale = true
			cache.Unlock()
		}
	}()
}

func fetch(conn *dbus.Conn) (State, error) {
	var objects map[dbus.ObjectPath]map[string]map[string]dbus.Variant
	err := conn.Object("org.bluez", "/").Call("org.freedesktop.DBus.ObjectManager.GetManagedObjects", 0).Store(&objects)
	if err != nil {
		return State{}, err
	}
	var s State
	for _, ifaces := range objects {
		if a, ok := ifaces["org.bluez.Adapter1"]; ok {
			s.Available = true
			s.Powered = s.Powered || boolOf(a["Powered"])
		}
		d, ok := ifaces["org.bluez.Device1"]
		if !ok || !boolOf(d["Paired"]) {
			continue
		}
		dev := Device{
			Address:   stringOf(d["Address"]),
			Name:      stringOf(d["Alias"]),
			Icon:      stringOf(d["Icon"]),
			Paired:    true,
			Connected: boolOf(d["Connected"]),
			Battery:   -1,
		}
		if dev.Name == "" {
			dev.Name = dev.Address
		}
		if b, ok := ifaces["org.bluez.Battery1"]; ok {
			if p, ok := b["Percentage"].Value().(byte); ok {
				dev.Battery = int(p)
			}
		}
		s.Devices = append(s.Devices, dev)
	}
	sort.Slice(s.Devices, func(i, j int) bool {
		if s.Devices[i].Connected != s.Devices[j].Connected {
			return s.Devices[i].Connected
		}
		return strings.ToLower(s.Devices[i].Name) < strings.ToLower(s.Devices[j].Name)
	})
	return s, nil
}

func boolOf(v dbus.Variant) bool {
	b, _ := v.Value().(bool)
	return b
}

func stringOf(v dbus.Variant) string {
	s, _ := v.Value().(string)
	return s
}
