package wifi

import (
	"fmt"

	"github.com/godbus/dbus/v5"
)

// iwd exports everything through one ObjectManager: devices, stations, the
// networks in range and the known ones. Signal strength comes from the
// station's ordered list, in hundredths of a dBm.
const iwdBus = "net.connman.iwd"

var iwd = backend{
	bus:        iwdBus,
	fetch:      iwdFetch,
	connect:    iwdConnect,
	disconnect: iwdDisconnect,
	scan: func(conn *dbus.Conn) {
		if station, err := iwdStation(conn); err == nil {
			conn.Object(iwdBus, station).Call("net.connman.iwd.Station.Scan", 0)
		}
	},
}

type managedObjects map[dbus.ObjectPath]map[string]map[string]dbus.Variant

func iwdObjects(conn *dbus.Conn) (managedObjects, error) {
	var objects managedObjects
	err := conn.Object(iwdBus, "/").Call("org.freedesktop.DBus.ObjectManager.GetManagedObjects", 0).Store(&objects)
	return objects, err
}

func iwdStation(conn *dbus.Conn) (dbus.ObjectPath, error) {
	objects, err := iwdObjects(conn)
	if err != nil {
		return "", err
	}
	for path, ifaces := range objects {
		if _, ok := ifaces["net.connman.iwd.Station"]; ok {
			return path, nil
		}
	}
	return "", fmt.Errorf("iwd has no station: is the device in station mode?")
}

func iwdFetch(conn *dbus.Conn) (State, error) {
	objects, err := iwdObjects(conn)
	if err != nil {
		return State{}, err
	}
	signal := map[dbus.ObjectPath]int{}
	var s State
	for path, ifaces := range objects {
		if dev, ok := ifaces["net.connman.iwd.Device"]; ok {
			if _, isStation := ifaces["net.connman.iwd.Station"]; isStation || s.Interface == "" {
				s.Interface = variantString(dev["Name"])
			}
		}
		if _, ok := ifaces["net.connman.iwd.Station"]; ok {
			var ordered []struct {
				Path   dbus.ObjectPath
				Signal int16
			}
			conn.Object(iwdBus, path).Call("net.connman.iwd.Station.GetOrderedNetworks", 0).Store(&ordered)
			for _, o := range ordered {
				signal[o.Path] = dbmPercent(int(o.Signal) / 100)
			}
		}
	}
	s.Networks = dedupe(iwdNetworks(objects, signal))
	return s, nil
}

// iwdNetworks reads the networks in range out of the managed objects.
func iwdNetworks(objects managedObjects, signal map[dbus.ObjectPath]int) []Network {
	var ns []Network
	for path, ifaces := range objects {
		n, ok := ifaces["net.connman.iwd.Network"]
		if !ok {
			continue
		}
		known := false
		if p, ok := n["KnownNetwork"].Value().(dbus.ObjectPath); ok && p != "" && p != "/" {
			known = true
		}
		ns = append(ns, Network{
			SSID:      variantString(n["Name"]),
			Security:  variantString(n["Type"]),
			Signal:    signal[path],
			Known:     known,
			Connected: variantBool(n["Connected"]),
		})
	}
	return ns
}

func iwdConnect(conn *dbus.Conn, ssid string) error {
	objects, err := iwdObjects(conn)
	if err != nil {
		return err
	}
	for path, ifaces := range objects {
		if n, ok := ifaces["net.connman.iwd.Network"]; ok && variantString(n["Name"]) == ssid {
			// blocks until iwd is connected or gives up
			return conn.Object(iwdBus, path).Call("net.connman.iwd.Network.Connect", 0).Err
		}
	}
	return fmt.Errorf("%s is not in range", ssid)
}

func iwdDisconnect(conn *dbus.Conn) error {
	station, err := iwdStation(conn)
	if err != nil {
		return err
	}
	return conn.Object(iwdBus, station).Call("net.connman.iwd.Station.Disconnect", 0).Err
}
