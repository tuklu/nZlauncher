package providers

import (
	"fmt"
	"strings"

	"github.com/tuklu/nZlauncher/internal/action"
	"github.com/tuklu/nZlauncher/internal/portable"
	"github.com/tuklu/nZlauncher/internal/shell"
	"github.com/tuklu/nZlauncher/internal/wifi"
)

// wifi / net: the radio toggle and the networks in range, from iwd or
// NetworkManager. Known and open networks connect directly; a new secured one
// opens a terminal where the service's own tool asks for the passphrase.
func Wifi(q string) []action.Action {
	if !On("wifi") {
		return nil
	}
	q = strings.ToLower(q)
	state := wifi.Current()
	wifi.Scan()

	var acts []action.Action
	if state.Enabled {
		acts = append(acts, action.Action{ID: "wifi:off", Name: "WiFi Off", Icon: "󰖪", Group: "wifi", Hint: "on",
			Cmd: "rfkill block wlan && notify-send -u low 'WiFi off'"})
	} else {
		acts = append(acts, action.Action{ID: "wifi:on", Name: "WiFi On", Icon: "󰖩", Group: "wifi", Hint: "off",
			Cmd: "rfkill unblock wlan && notify-send -u low 'WiFi on'"})
	}

	shown := 0
	for _, n := range state.Networks {
		if q != "" && !strings.Contains(strings.ToLower(n.SSID), q) {
			continue
		}
		if shown++; shown > 12 {
			break
		}
		ssid := shell.Quote(n.SSID)
		a := action.Action{ID: "wifi:" + n.SSID, Icon: signalIcon(n.Signal), Group: "wifi", Hint: wifiHint(n)}
		switch {
		case n.Connected:
			a.Name = "Disconnect " + n.SSID
			a.Cmd = "nzlauncher wifi disconnect && notify-send -u low " + shell.Quote("Disconnected "+n.SSID) +
				" || notify-send " + shell.Quote("Could not disconnect "+n.SSID)
		case n.Known || n.Security == "open":
			a.Name = "Connect " + n.SSID
			a.Cmd = "nzlauncher wifi connect " + ssid + " && notify-send -u low " + shell.Quote("Connected "+n.SSID) +
				" || notify-send " + shell.Quote("Could not connect "+n.SSID)
		default:
			a.Name = "Connect " + n.SSID
			a.Cmd = portable.Terminal(joinCommand(state, n.SSID))
			if a.Cmd == "" {
				continue // no terminal to ask for the passphrase in
			}
		}
		acts = append(acts, a)
	}

	switch state.Backend {
	case "networkmanager":
		return append(acts, tuiTail("nmtui", "nmtui", "full WiFi manager")...)
	}
	return append(acts, tuiTail("impala", "Impala", "full WiFi manager")...)
}

// joinCommand asks for the passphrase in a terminal, with the backend's tool.
func joinCommand(state wifi.State, ssid string) string {
	if state.Backend == "networkmanager" {
		return "nmcli --ask device wifi connect " + shell.Quote(ssid)
	}
	return "iwctl station " + shell.Quote(state.Interface) + " connect " + shell.Quote(ssid)
}

func wifiHint(n wifi.Network) string {
	switch {
	case n.Connected:
		return fmt.Sprintf("connected · %d%%", n.Signal)
	case n.Known:
		return fmt.Sprintf("known · %d%%", n.Signal)
	case n.Security == "open":
		return fmt.Sprintf("open · %d%%", n.Signal)
	}
	return fmt.Sprintf("new · %s · %d%%", n.Security, n.Signal)
}

func signalIcon(percent int) string {
	switch {
	case percent >= 75:
		return "󰤨"
	case percent >= 50:
		return "󰤥"
	case percent >= 25:
		return "󰤢"
	}
	return "󰤟"
}

// Escape hatch into a real TUI when the quick action isn't enough.
func tuiTail(binary, name, desc string) []action.Action {
	if !portable.Have(binary) || portable.Terminal(binary) == "" {
		return nil
	}
	return []action.Action{{
		ID: "tui:" + binary, Name: "Open " + name + "   (" + desc + ")",
		Icon: "", Group: "tui",
		Cmd: portable.Terminal(binary),
	}}
}
