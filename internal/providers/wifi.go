package providers

import (
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	"github.com/tuklu/nZlauncher/internal/action"
	"github.com/tuklu/nZlauncher/internal/shell"
	"github.com/tuklu/nZlauncher/internal/sysinfo"
)

// wifi / net — toggles, known networks from iwd's cached scan, impala for depth.
// Known networks connect directly; new ones need a passphrase, which is what
// the impala escape hatch is for.
func Wifi(q string) []action.Action {
	if !On("wifi") {
		return nil
	}
	q = strings.ToLower(q)
	iface := wifiInterface()

	acts := []action.Action{
		{ID: "wifi:on", Name: "WiFi On", Icon: "󰖩", Group: "wifi",
			Cmd: "rfkill unblock wlan && notify-send -u low 'WiFi on'"},
		{ID: "wifi:off", Name: "WiFi Off", Icon: "󰖪", Group: "wifi",
			Cmd: "rfkill block wlan && notify-send -u low 'WiFi off'"},
	}

	connected := sysinfo.CurrentSSID()
	for _, ssid := range scanNetworks(iface) {
		if ssid == connected {
			acts = append(acts, action.Action{
				ID: "wifi:" + ssid, Name: "Disconnect " + ssid + "   (connected)",
				Icon: "󰖩", Group: "wifi",
				Cmd: "iwctl station " + iface + " disconnect",
			})
			continue
		}
		acts = append(acts, action.Action{
			ID: "wifi:" + ssid, Name: "Connect " + ssid, Icon: "󰖩", Group: "wifi",
			Cmd: "iwctl station " + iface + " connect " + shell.Quote(ssid) +
				" || notify-send 'Needs passphrase' 'Open impala to join " + ssid + "'",
		})
	}

	if q != "" {
		filtered := acts[:0]
		for _, a := range acts {
			if strings.Contains(strings.ToLower(a.Name), q) {
				filtered = append(filtered, a)
			}
		}
		acts = filtered
	}

	return append(acts, tuiTail("impala", "Impala", "full WiFi manager")...)
}

var ansiCodes = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func scanNetworks(iface string) []string {
	out, err := exec.Command("iwctl", "station", iface, "get-networks", "rssi-dbms").Output()
	if err != nil {
		return nil
	}

	var ssids []string
	for _, line := range strings.Split(ansiCodes.ReplaceAllString(string(out), ""), "\n") {
		fields := strings.Fields(strings.TrimPrefix(strings.TrimSpace(line), "> "))
		// columns: ssid (may contain spaces), security, rssi — header rows
		// and separators fail the numeric-rssi check
		if len(fields) < 3 || !strings.HasPrefix(fields[len(fields)-1], "-") {
			continue
		}
		if _, err := strconv.Atoi(fields[len(fields)-1]); err != nil {
			continue
		}
		ssids = append(ssids, strings.Join(fields[:len(fields)-2], " "))
	}
	return ssids
}

func wifiInterface() string {
	out, err := exec.Command("iw", "dev").Output()
	if err != nil {
		return "wlan0"
	}
	for _, line := range strings.Split(string(out), "\n") {
		if name, ok := strings.CutPrefix(strings.TrimSpace(line), "Interface "); ok {
			return name
		}
	}
	return "wlan0"
}

// Escape hatch into a real TUI when the quick action isn't enough.
func tuiTail(binary, name, desc string) []action.Action {
	if _, err := exec.LookPath(binary); err != nil {
		return nil
	}
	return []action.Action{{
		ID: "tui:" + binary, Name: "Open " + name + "   (" + desc + ")",
		Icon: "", Group: "tui",
		Cmd: "launch-tui " + binary,
	}}
}
