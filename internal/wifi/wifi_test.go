package wifi

import (
	"os"
	"testing"

	"github.com/godbus/dbus/v5"
)

func TestIwdNetworks(t *testing.T) {
	objects := managedObjects{
		"/net/connman/iwd/0/3/686f6d65_psk": {"net.connman.iwd.Network": {
			"Name": dbus.MakeVariant("home"), "Type": dbus.MakeVariant("psk"), "Connected": dbus.MakeVariant(true),
			"KnownNetwork": dbus.MakeVariant(dbus.ObjectPath("/net/connman/iwd/686f6d65_psk")),
		}},
		"/net/connman/iwd/0/3/63616665_open": {"net.connman.iwd.Network": {
			"Name": dbus.MakeVariant("cafe"), "Type": dbus.MakeVariant("open"), "Connected": dbus.MakeVariant(false),
		}},
	}
	signal := map[dbus.ObjectPath]int{"/net/connman/iwd/0/3/686f6d65_psk": 80, "/net/connman/iwd/0/3/63616665_open": 40}
	ns := iwdNetworks(objects, signal)
	sortNetworks(ns)
	if len(ns) != 2 || ns[0] != (Network{"home", "psk", 80, true, true}) || ns[1] != (Network{"cafe", "open", 40, false, false}) {
		t.Fatalf("%+v", ns)
	}
}

func TestNMNetworks(t *testing.T) {
	points := []accessPoint{
		{ssid: "home", strength: 70, flags: 1, rsn: 0x100, active: true},
		{ssid: "home", strength: 30, flags: 1, rsn: 0x100}, // second access point of the same network
		{ssid: "office", strength: 90, flags: 1, rsn: 0x200},
		{ssid: "cafe", strength: 50},
		{ssid: "", strength: 99}, // hidden
	}
	ns := dedupe(nmNetworks(points, map[string]dbus.ObjectPath{"home": "/p/1"}))
	sortNetworks(ns)
	want := []Network{
		{"home", "psk", 70, true, true},
		{"office", "8021x", 90, false, false},
		{"cafe", "open", 50, false, false},
	}
	if len(ns) != len(want) {
		t.Fatalf("%+v", ns)
	}
	for i := range want {
		if ns[i] != want[i] {
			t.Errorf("%d: %+v, want %+v", i, ns[i], want[i])
		}
	}
}

func TestDbmPercent(t *testing.T) {
	for dbm, want := range map[int]int{-100: 0, -75: 50, -50: 100, -30: 100, -110: 0} {
		if got := dbmPercent(dbm); got != want {
			t.Errorf("%d dBm: %d, want %d", dbm, got, want)
		}
	}
}

// Run against the real machine: NZ_WIFI_LIVE=1 go test ./internal/wifi -run Live -v
func TestLive(t *testing.T) {
	if os.Getenv("NZ_WIFI_LIVE") == "" {
		t.Skip("set NZ_WIFI_LIVE=1")
	}
	s := Current()
	t.Logf("backend %s, %s, enabled %v, connected %q", s.Backend, s.Interface, s.Enabled, s.Connected())
	for _, n := range s.Networks {
		t.Logf("  %+v", n)
	}
}

func BenchmarkCurrent(b *testing.B) {
	Current()
	for b.Loop() {
		Current()
	}
}
