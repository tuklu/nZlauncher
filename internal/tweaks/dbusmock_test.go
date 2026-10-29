//go:build dbusmock

package tweaks

import (
	"slices"
	"testing"
)

// Against python-dbusmock's power-profiles daemon, started by
// test/dbusmock/run.py.
func TestPowerProfilesMock(t *testing.T) {
	s, ok := Power()
	if !ok {
		t.Fatal("no power profile service found")
	}
	if s.Active != "balanced" || !slices.Equal(s.Profiles, []string{"power-saver", "balanced", "performance"}) {
		t.Fatalf("state %+v", s)
	}
	if err := SetPower("performance"); err != nil {
		t.Fatal(err)
	}
	if s, _ := Power(); s.Active != "performance" {
		t.Errorf("after setting performance: %q", s.Active)
	}
	if err := SetPower("turbo"); err == nil {
		t.Errorf("a profile the machine lacks was accepted")
	}
}
