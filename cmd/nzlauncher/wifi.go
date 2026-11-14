package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/tuklu/nZlauncher/internal/config"
	"github.com/tuklu/nZlauncher/internal/platform"
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
	ask, ok := passphraseDialog()
	if !ok {
		fmt.Fprintln(os.Stderr, "nzlauncher: no passphrase dialog (zenity or kdialog) on this display")
		os.Exit(exitNeedsTerminal)
	}
	err := wifi.Join(ssid, ask)
	switch {
	case errors.Is(err, wifi.ErrNoAgent):
		fmt.Fprintln(os.Stderr, "nzlauncher:", err)
		os.Exit(exitNeedsTerminal)
	case errors.Is(err, wifi.ErrCanceled):
		os.Exit(exitCanceled)
	}
	return err
}

// passphraseDialog finds a password dialog for the graphical session.
func passphraseDialog() (wifi.Ask, bool) {
	if os.Getenv("WAYLAND_DISPLAY") == "" && os.Getenv("DISPLAY") == "" {
		return nil, false
	}
	dialogs := []struct {
		name string
		args func(prompt string) []string
	}{
		{"zenity", func(p string) []string { return []string{"--password", "--title", p} }},
		{"kdialog", func(p string) []string { return []string{"--password", p, "--title", "Wi-Fi"} }},
	}
	for _, d := range dialogs {
		path, err := exec.LookPath(d.name)
		if err != nil {
			continue
		}
		return func(prompt string) (string, error) {
			out, err := exec.Command(path, d.args(prompt)...).Output()
			if err != nil {
				return "", wifi.ErrCanceled
			}
			return strings.TrimSuffix(string(out), "\n"), nil
		}, true
	}
	return nil, false
}
