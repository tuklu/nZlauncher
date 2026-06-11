package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func onBattery() bool {
	online, _ := filepath.Glob("/sys/class/power_supply/A*/online")
	for _, f := range online {
		if data, err := os.ReadFile(f); err == nil && strings.TrimSpace(string(data)) == "1" {
			return false
		}
	}
	bats, _ := filepath.Glob("/sys/class/power_supply/BAT*")
	return len(bats) > 0
}

func currentSSID() string {
	out, err := exec.Command("iw", "dev").Output()
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(out), "\n") {
		if ssid, ok := strings.CutPrefix(strings.TrimSpace(line), "ssid "); ok {
			return ssid
		}
	}
	return ""
}
