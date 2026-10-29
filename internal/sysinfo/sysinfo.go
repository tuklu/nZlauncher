// Package sysinfo reads the machine state that launches are tagged with.
package sysinfo

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/tuklu/nZlauncher/internal/wifi"
)

func OnBattery() bool {
	online, _ := filepath.Glob("/sys/class/power_supply/A*/online")
	for _, f := range online {
		if data, err := os.ReadFile(f); err == nil && strings.TrimSpace(string(data)) == "1" {
			return false
		}
	}
	bats, _ := filepath.Glob("/sys/class/power_supply/BAT*")
	return len(bats) > 0
}

// CurrentSSID is the Wi-Fi network in use, from the Wi-Fi service's cached
// state; "" when offline or no supported service runs.
func CurrentSSID() string {
	return wifi.Current().Connected()
}
