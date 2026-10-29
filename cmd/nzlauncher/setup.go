package main

import (
	"fmt"

	"github.com/tuklu/nZlauncher/internal/config"
	"github.com/tuklu/nZlauncher/internal/platform"
)

// runSetup prints what nzlauncher found on this machine and what drives
// each capability, overrides from config.toml included.
func runSetup() error {
	settings, _ := config.LoadSettings()
	platform.Configure(settings.Backends)
	detected, used := platform.Detect(), platform.Current()
	row := func(name, found, chosen string) {
		if found == "" {
			found = "none found"
		}
		note := ""
		if chosen != found && chosen != "" {
			note = "   (config.toml: " + chosen + ")"
		}
		fmt.Printf("%-14s %s%s\n", name, found, note)
	}
	row("compositor", detected.Compositor, used.Compositor)
	row("wifi", detected.Wifi, used.Wifi)
	row("power", detected.Power, used.Power)
	row("notifications", detected.Notifications, used.Notifications)
	immutable := ""
	if detected.Immutable {
		immutable = ", immutable: tweaks stay runtime only"
	}
	fmt.Printf("%-14s %s%s\n", "distro", detected.Distro, immutable)
	return nil
}
