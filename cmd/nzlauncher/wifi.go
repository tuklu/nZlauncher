package main

import (
	"fmt"

	"github.com/tuklu/nZlauncher/internal/config"
	"github.com/tuklu/nZlauncher/internal/platform"
	"github.com/tuklu/nZlauncher/internal/wifi"
)

// runWifi is what the Wi-Fi results run: connect and disconnect through
// whichever service drives Wi-Fi here.
func runWifi(args []string) error {
	settings, _ := config.LoadSettings()
	platform.Configure(settings.Backends)
	switch {
	case len(args) == 2 && args[0] == "connect":
		return wifi.Connect(args[1])
	case len(args) == 1 && args[0] == "disconnect":
		return wifi.Disconnect()
	}
	return fmt.Errorf("usage: nzlauncher wifi connect <ssid> | disconnect")
}
