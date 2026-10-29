//go:build dbusmock

package bluez

import "testing"

// Against python-dbusmock's BlueZ: one adapter, one paired mouse, started
// by test/dbusmock/run.py.
func TestBlueZMock(t *testing.T) {
	s := Current()
	if !s.Available {
		t.Fatal("no adapter seen")
	}
	if len(s.Devices) != 1 || s.Devices[0].Name != "MX Master" || !s.Devices[0].Paired {
		t.Fatalf("devices %+v", s.Devices)
	}
}
