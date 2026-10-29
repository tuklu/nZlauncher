package providers

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"github.com/tuklu/nZlauncher/internal/action"
	"github.com/tuklu/nZlauncher/internal/shell"
)

// bt / bluetooth — power toggle plus paired devices with live connect state.
func Bluetooth(q string) []action.Action {
	if !On("bluetooth") {
		return nil
	}
	q = strings.ToLower(q)
	if _, err := exec.LookPath("bluetoothctl"); err != nil {
		return nil
	}

	var acts []action.Action
	if !btPowered() {
		return append(acts, action.Action{
			ID: "bt:power", Name: "Bluetooth On", Icon: "󰂯", Group: "bluetooth",
			Cmd: "bluetoothctl power on && notify-send -u low 'Bluetooth on'",
		})
	}
	acts = append(acts, action.Action{
		ID: "bt:power", Name: "Bluetooth Off", Icon: "󰂲", Group: "bluetooth",
		Cmd: "bluetoothctl power off && notify-send -u low 'Bluetooth off'",
	})

	connected := map[string]bool{}
	for mac := range btDevices("Connected") {
		connected[mac] = true
	}
	for mac, name := range btDevices("") {
		if q != "" && !strings.Contains(strings.ToLower(name), q) {
			continue
		}
		if connected[mac] {
			acts = append(acts, action.Action{
				ID: "bt:" + mac, Name: "Disconnect " + name, Icon: "󰂱", Group: "bluetooth",
				Cmd: "bluetoothctl disconnect " + mac,
			})
		} else {
			acts = append(acts, action.Action{
				ID: "bt:" + mac, Name: "Connect " + name, Icon: "󰂯", Group: "bluetooth",
				Cmd: "bluetoothctl connect " + mac,
			})
		}
	}
	return append(acts, tuiTail("bluetui", "Bluetui", "full Bluetooth manager")...)
}

func btPowered() bool {
	out, err := exec.Command("bluetoothctl", "show").Output()
	return err == nil && strings.Contains(string(out), "Powered: yes")
}

func btDevices(filter string) map[string]string {
	args := []string{"devices"}
	if filter != "" {
		args = append(args, filter)
	}
	out, err := exec.Command("bluetoothctl", args...).Output()
	if err != nil {
		return nil
	}
	devices := map[string]string{}
	for _, line := range strings.Split(string(out), "\n") {
		parts := strings.SplitN(strings.TrimSpace(line), " ", 3)
		if len(parts) == 3 && parts[0] == "Device" {
			devices[parts[1]] = parts[2]
		}
	}
	return devices
}

// audio / sound / output — pick a specific output device, current one marked.
func Audio(q string) []action.Action {
	if !On("audio") {
		return nil
	}
	q = strings.ToLower(q)
	out, err := exec.Command("pactl", "-f", "json", "list", "sinks").Output()
	if err != nil {
		return nil
	}
	var sinks []struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if json.Unmarshal(out, &sinks) != nil {
		return nil
	}
	current := ""
	if d, err := exec.Command("pactl", "get-default-sink").Output(); err == nil {
		current = strings.TrimSpace(string(d))
	}

	var acts []action.Action
	for _, s := range sinks {
		if q != "" && !strings.Contains(strings.ToLower(s.Description), q) {
			continue
		}
		name := "Output: " + s.Description
		if s.Name == current {
			name += "   (current)"
		}
		acts = append(acts, action.Action{
			ID: "sink:" + s.Name, Name: name, Icon: "󰓃", Group: "audio",
			Cmd: "pactl set-default-sink " + shell.Quote(s.Name),
		})
	}
	return append(acts, tuiTail("wiremix", "Wiremix", "full audio mixer")...)
}

// vol / volume — direct percentage or presets, plus mute toggle.
func Volume(q string) []action.Action {
	if !On("volume") {
		return nil
	}
	if n, err := strconv.Atoi(q); err == nil && n >= 0 && n <= 100 {
		return []action.Action{{
			ID: "vol:set", Name: fmt.Sprintf("Volume %d%%", n), Icon: "󰕾", Group: "audio",
			Cmd: fmt.Sprintf("wpctl set-volume @DEFAULT_AUDIO_SINK@ %d%%", n),
		}}
	}
	var acts []action.Action
	for _, n := range []int{25, 50, 75, 100} {
		acts = append(acts, action.Action{
			ID: fmt.Sprintf("vol:%d", n), Name: fmt.Sprintf("Volume %d%%", n), Icon: "󰕾", Group: "audio",
			Cmd: fmt.Sprintf("wpctl set-volume @DEFAULT_AUDIO_SINK@ %d%%", n),
		})
	}
	return append(acts, action.Action{
		ID: "vol:mute", Name: "Mute Toggle", Icon: "󰝟", Group: "audio",
		Cmd: "osd --output-volume mute-toggle",
	})
}

// ws / workspace — jump or move the active window; "ws 3", "ws move 3".
func Workspace(q string) []action.Action {
	if !On("workspace") {
		return nil
	}
	move := false
	if rest, ok := PrefixArg(q, "move"); ok {
		move, q = true, rest
	}

	target := func(n int) []action.Action {
		var acts []action.Action
		if !move {
			acts = append(acts, action.Action{
				ID: fmt.Sprintf("ws:%d", n), Name: fmt.Sprintf("Go to Workspace %d", n),
				Icon: "󱂬", Group: "workspace",
				Cmd: fmt.Sprintf("hyprctl dispatch workspace %d", n),
			})
		}
		acts = append(acts, action.Action{
			ID: fmt.Sprintf("ws:move:%d", n), Name: fmt.Sprintf("Move Window to Workspace %d", n),
			Icon: "󱂬", Group: "workspace",
			Cmd: fmt.Sprintf("hyprctl dispatch movetoworkspace %d", n),
		})
		return acts
	}

	if n, err := strconv.Atoi(q); err == nil && n >= 1 && n <= 10 {
		return target(n)
	}
	var acts []action.Action
	for n := 1; n <= 5; n++ {
		acts = append(acts, target(n)...)
	}
	return acts
}

// bright / brightness — direct percentage or presets, via the brightness script.
func Brightness(q string) []action.Action {
	if !On("brightness") {
		return nil
	}
	if n, err := strconv.Atoi(q); err == nil && n >= 1 && n <= 100 {
		return []action.Action{{
			ID: "bright:set", Name: fmt.Sprintf("Brightness %d%%", n), Icon: "󰃟", Group: "display",
			Cmd: fmt.Sprintf("brightness %d%%", n),
		}}
	}
	var acts []action.Action
	for _, n := range []int{10, 30, 60, 100} {
		acts = append(acts, action.Action{
			ID: fmt.Sprintf("bright:%d", n), Name: fmt.Sprintf("Brightness %d%%", n), Icon: "󰃟", Group: "display",
			Cmd: fmt.Sprintf("brightness %d%%", n),
		})
	}
	return acts
}
