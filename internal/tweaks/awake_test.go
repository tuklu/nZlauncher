package tweaks

import "testing"

// Specs the daemon must refuse before touching any bus.
func TestApplyAwakeRejects(t *testing.T) {
	for _, spec := range []string{"", "soon", "-5", "screen:", "screen:soon", "lid:60", "all:-1"} {
		if err := ApplyAwake(spec); err == nil {
			t.Errorf("%q was accepted", spec)
		}
	}
}

// Releasing what is not held is not an error, whatever the machine has.
func TestReleaseNothingHeld(t *testing.T) {
	for _, spec := range []string{"off", "screen:off", "sleep:off", "all:off"} {
		if err := ApplyAwake(spec); err != nil {
			t.Errorf("%q: %v", spec, err)
		}
	}
}
