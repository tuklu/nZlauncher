package engine

import (
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// Needs a running Hyprland and ghostty; opens and closes one window.
func TestWindowListFollowsHyprland(t *testing.T) {
	if os.Getenv("HYPRLAND_INSTANCE_SIGNATURE") == "" {
		t.Skip("not under Hyprland")
	}
	const title = "nzlauncher-window-test"
	has := func() bool {
		for _, a := range cache.windowList() {
			if strings.Contains(a.Name, title) {
				return true
			}
		}
		return false
	}
	waitFor := func(want bool) bool {
		for range 50 {
			if has() == want {
				return true
			}
			time.Sleep(100 * time.Millisecond)
		}
		return false
	}

	cache.windowList() // start watching
	win := exec.Command("ghostty", "--gtk-single-instance=false", "--title="+title, "-e", "sleep", "30")
	if err := win.Start(); err != nil {
		t.Skip("ghostty not available")
	}
	defer win.Process.Kill()

	if !waitFor(true) {
		t.Fatal("opened window never showed up in the cached list")
	}
	win.Process.Kill()
	win.Wait()
	if !waitFor(false) {
		t.Fatal("closed window stayed in the cached list")
	}
}
