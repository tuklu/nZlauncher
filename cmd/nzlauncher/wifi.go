package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/tuklu/nZlauncher/internal/config"
	"github.com/tuklu/nZlauncher/internal/platform"
	"github.com/tuklu/nZlauncher/internal/prompt"
	"github.com/tuklu/nZlauncher/internal/wifi"
)

// Exit codes of wifi join, read by the Wi-Fi results: anything else is a
// failed connection.
const (
	exitNeedsTerminal = 3 // no dialog or no agent here: use the service's own tool
	exitCanceled      = 4 // the passphrase dialog was closed
)

// runWifi is what the Wi-Fi results run: connect and disconnect through
// whichever service drives Wi-Fi here, and join new secured networks with a
// passphrase dialog.
func runWifi(args []string) error {
	settings, _ := config.LoadSettings()
	platform.Configure(settings.Backends)
	switch {
	case len(args) == 2 && args[0] == "connect":
		return wifi.Connect(args[1])
	case len(args) == 2 && args[0] == "join":
		return runJoin(args[1])
	case len(args) == 1 && args[0] == "disconnect":
		return wifi.Disconnect()
	case len(args) == 1 && args[0] == "list":
		for _, n := range wifi.Current().Networks {
			fmt.Println(n.SSID)
		}
		return nil
	}
	return misuse("wifi", "wifi takes connect, join, disconnect or list")
}

func runJoin(ssid string) error {
	dialog, ok := prompt.Dialog()
	if !ok {
		fmt.Fprintln(os.Stderr, "nzlauncher: no passphrase dialog (zenity or kdialog) on this display")
		os.Exit(exitNeedsTerminal)
	}
	err := wifi.Join(ssid, func(title string) (string, error) {
		answer, err := dialog(title)
		if errors.Is(err, prompt.ErrCanceled) {
			return "", wifi.ErrCanceled
		}
		return answer, err
	})
	switch {
	case errors.Is(err, wifi.ErrNoAgent):
		fmt.Fprintln(os.Stderr, "nzlauncher:", err)
		os.Exit(exitNeedsTerminal)
	case errors.Is(err, wifi.ErrCanceled):
		os.Exit(exitCanceled)
	}
	return err
}
