package main

import (
	"errors"
	"strconv"

	"github.com/tuklu/nZlauncher/internal/config"
	"github.com/tuklu/nZlauncher/internal/platform"
	"github.com/tuklu/nZlauncher/internal/server"
	"github.com/tuklu/nZlauncher/internal/tweaks"
)

// runTweak is what the tweak results run.
func runTweak(args []string) error {
	settings, _ := config.LoadSettings()
	platform.Configure(settings.Backends)
	switch {
	case len(args) == 1 && args[0] == "normal":
		return backToNormal()
	case len(args) == 3 && args[0] == "awake":
		return askAwake(args[1] + ":" + awakeValue(args[2]))
	case len(args) != 2:
		return misuse("tweak", "tweak takes a setting and a value")
	}
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
			return misuse("tweak", "nightlight takes off or 1000 to 10000 kelvin")
		}
		return tweaks.SetNightlight(kelvin)
	case "awake":
		return askAwake(awakeValue(value))
	}
	return misuse("tweak", "no setting %q", what)
}

func awakeValue(v string) string {
	if v == "on" {
		return "0"
	}
	return v
}

// askAwake hands a keep-awake request to the daemon: an inhibit lives as long
// as whoever holds it.
func askAwake(spec string) error {
	resp, err := server.Ask(server.Request{Op: "awake", Q: spec})
	if err != nil && resp.Error == "" {
		return errors.New("keep awake needs the daemon: nzlauncher serve")
	}
	return err
}

// backToNormal lets go of every hold, then picks the balanced profile where
// there are power profiles. Either half can be missing on a machine.
func backToNormal() error {
	awakeErr := askAwake("off")
	if _, ok := tweaks.Power(); !ok {
		return awakeErr
	}
	return errors.Join(awakeErr, tweaks.SetPower("balanced"))
}
