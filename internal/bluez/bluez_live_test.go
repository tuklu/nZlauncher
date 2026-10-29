package bluez

import (
	"os"
	"os/exec"
	"testing"
	"time"
)

// Opt-in: flips the adapter off and back on. NZ_BT_LIVE=1 go test ./internal/bluez
func TestFollowsPower(t *testing.T) {
	if os.Getenv("NZ_BT_LIVE") == "" {
		t.Skip("set NZ_BT_LIVE=1 to toggle the real adapter")
	}
	start := Current()
	if !start.Available || !start.Powered {
		t.Skip("needs a powered adapter")
	}
	defer exec.Command("bluetoothctl", "power", "on").Run()
	waitFor := func(want bool) time.Duration {
		begin := time.Now()
		for time.Since(begin) < 5*time.Second {
			if Current().Powered == want {
				return time.Since(begin)
			}
			time.Sleep(10 * time.Millisecond)
		}
		t.Fatalf("cache never saw Powered=%v", want)
		return 0
	}
	exec.Command("bluetoothctl", "power", "off").Run()
	t.Logf("saw power off after %v", waitFor(false))
	exec.Command("bluetoothctl", "power", "on").Run()
	t.Logf("saw power on after %v", waitFor(true))
}
