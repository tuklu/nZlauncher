package main

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/tuklu/nZlauncher/internal/config"
	"github.com/tuklu/nZlauncher/internal/platform"
	"github.com/tuklu/nZlauncher/internal/server"
	"github.com/tuklu/nZlauncher/internal/tweaks"
)

const tweakUsage = "usage: nzlauncher tweak power <profile> | dnd on|off | nightlight <kelvin>|off | awake on|off|<seconds>"

// runTweak is what the tweak results run.
func runTweak(args []string) error {
	if len(args) != 2 {
		return errors.New(tweakUsage)
	}
	settings, _ := config.LoadSettings()
	platform.Configure(settings.Backends)
	what, value := args[0], args[1]
	switch what {
	case "power":
		return tweaks.SetPower(value)
	case "dnd":
		return tweaks.SetDND(value == "on")
	case "nightlight":
		if value == "off" {
			return tweaks.SetNightlight(0)
		}
		kelvin, err := strconv.Atoi(value)
		if err != nil || kelvin < 1000 || kelvin > 10000 {
			return fmt.Errorf("nightlight takes off or 1000 to 10000 kelvin")
		}
		return tweaks.SetNightlight(kelvin)
	case "awake":
		// the inhibit lives as long as whoever holds it, so the daemon does
		q := value
		if value == "on" {
			q = "0"
		}
		resp, err := server.Ask(server.Request{Op: "awake", Q: q})
		if err != nil && resp.Error == "" {
			return errors.New("keep awake needs the daemon: nzlauncher serve")
		}
		return err
	}
	return errors.New(tweakUsage)
}
