//go:build dbusmock

package wifi

import (
	"slices"
	"testing"
)

// Against python-dbusmock's NetworkManager: test/dbusmock/run.py sets the
// scene (home saved and in use, office 802.1X, cafe open).
func TestNetworkManagerMock(t *testing.T) {
	s := Current()
	if s.Backend != "networkmanager" || s.Interface != "wlan0" {
		t.Fatalf("backend %q on %q", s.Backend, s.Interface)
	}
	want := []Network{
		{"home", "psk", 80, true, true},
		{"office", "8021x", 90, false, false},
		{"cafe", "open", 40, false, false},
	}
	if !slices.Equal(s.Networks, want) {
		t.Fatalf("networks %+v, want %+v", s.Networks, want)
	}
	if err := Connect("home"); err != nil {
		t.Errorf("connect to a saved network: %v", err)
	}
	if err := Connect("cafe"); err != nil {
		t.Errorf("connect to an open network: %v", err)
	}
	if err := Connect("office"); err == nil {
		t.Errorf("a new secured network connected without a passphrase")
	}
	if err := Disconnect(); err != nil {
		t.Errorf("disconnect: %v", err)
	}
}
