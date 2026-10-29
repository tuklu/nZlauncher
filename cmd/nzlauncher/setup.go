package main

import (
	"fmt"
	"os/exec"

	"github.com/tuklu/nZlauncher/internal/config"
	"github.com/tuklu/nZlauncher/internal/platform"
	"github.com/tuklu/nZlauncher/internal/portable"
)

// runSetup prints what nzlauncher found on this machine and what drives
// each capability, overrides from config.toml included.
func runSetup() error {
	settings, _ := config.LoadSettings()
	platform.Configure(settings.Backends)
	portable.Configure(settings.Commands)
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
	row("nightlight", detected.Nightlight, used.Nightlight)
	row("idle", detected.Idle, used.Idle)
	if _, err := exec.LookPath("tlp"); err == nil && detected.Power == "" {
		fmt.Println("               TLP is installed: its tlp-pd package adds power profile switching")
	}
	immutable := ""
	if detected.Immutable {
		immutable = ", immutable: tweaks stay runtime only"
	}
	fmt.Printf("%-14s %s%s\n", "distro", detected.Distro, immutable)

	fmt.Println("\ncommands (set in [commands] in config.toml)")
	for _, r := range portable.Explain() {
		fmt.Printf("  %-16s %-58s %s\n", r.Key, r.Command, "("+r.Source+")")
	}
	return nil
}
