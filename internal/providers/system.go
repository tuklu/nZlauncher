package providers

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"github.com/tuklu/nZlauncher/internal/action"
	"github.com/tuklu/nZlauncher/internal/bluez"
	"github.com/tuklu/nZlauncher/internal/portable"
	"github.com/tuklu/nZlauncher/internal/shell"
)

// bt / bluetooth: adapter power and every paired device, read from BlueZ.
// Devices show while the adapter is off too: connecting turns it on first.
func Bluetooth(q string) []action.Action {
	if !On("bluetooth") {
		return nil
	}
	state := bluez.Current()
	if !state.Available {
		return nil
	}
	q = strings.ToLower(q)

	var acts []action.Action
	if state.Powered {
		acts = append(acts, action.Action{
			ID: "bt:power", Name: "Bluetooth Off", Icon: "󰂲", Group: "bluetooth", Hint: "on",
			Cmd: "bluetoothctl power off >/dev/null && notify-send -u low 'Bluetooth off'",
		})
	} else {
		acts = append(acts, action.Action{
			ID: "bt:power", Name: "Bluetooth On", Icon: "󰂯", Group: "bluetooth", Hint: "off",
			Cmd: "bluetoothctl power on >/dev/null && notify-send -u low 'Bluetooth on'",
		})
	}

	for _, d := range state.Devices {
		if q != "" && !strings.Contains(strings.ToLower(d.Name), q) {
			continue
		}
		name, mac := d.Name, shell.Quote(d.Address)
		a := action.Action{ID: "bt:" + d.Address, Icon: btIcon(d.Icon), Group: "bluetooth", Hint: btHint(d)}
		if d.Connected {
			a.Name = "Disconnect " + name
			a.Cmd = "bluetoothctl disconnect " + mac + " >/dev/null && notify-send -u low " + shell.Quote("Disconnected "+name) +
				" || notify-send " + shell.Quote("Could not disconnect "+name)
		} else {
			a.Name = "Connect " + name
			power := ""
			if !state.Powered {
				power = "bluetoothctl power on >/dev/null && "
			}
			a.Cmd = power + "bluetoothctl connect " + mac + " >/dev/null && notify-send -u low " + shell.Quote("Connected "+name) +
				" || notify-send " + shell.Quote("Could not connect "+name)
		}
		acts = append(acts, a)
	}
	return append(acts, tuiTail("bluetui", "Bluetui", "full Bluetooth manager")...)
}

func btHint(d bluez.Device) string {
	hint := "paired"
	if d.Connected {
		hint = "connected"
	}
	if d.Battery >= 0 {
		hint += fmt.Sprintf(" · %d%%", d.Battery)
	}
	return hint
}

// BlueZ reports a freedesktop icon name for the device class.
func btIcon(kind string) string {
	switch {
	case strings.Contains(kind, "mouse"):
		return "󰍽"
	case strings.Contains(kind, "keyboard"):
		return "󰌌"
	case strings.Contains(kind, "headset"), strings.Contains(kind, "headphone"):
		return "󰋋"
	case strings.HasPrefix(kind, "audio"):
		return "󰓃"
	case strings.Contains(kind, "phone"):
		return "󰏲"
	case strings.Contains(kind, "gaming"):
		return "󰊗"
	}
	return "󰂯"
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
	switch sign, n, ok := level(q); {
	case q == "mute":
		return []action.Action{muteAction()}
	case q == "unmute":
		return []action.Action{unmuteAction()}
	case ok && sign == "+":
		return []action.Action{{ID: "vol:up", Name: fmt.Sprintf("Volume Up %d%%", n), Icon: "󰕾", Group: "audio",
			Cmd: portable.VolumeStep(n)}}
	case ok && sign == "-":
		return []action.Action{{ID: "vol:down", Name: fmt.Sprintf("Volume Down %d%%", n), Icon: "󰖀", Group: "audio",
			Cmd: portable.VolumeStep(-n)}}
	case ok:
		return []action.Action{{
			ID: "vol:set", Name: fmt.Sprintf("Volume %d%%", n), Icon: "󰕾", Group: "audio",
			Cmd: portable.Volume(n),
		}}
	}
	var acts []action.Action
	for _, n := range []int{25, 50, 75, 100} {
		acts = append(acts, action.Action{
			ID: fmt.Sprintf("vol:%d", n), Name: fmt.Sprintf("Volume %d%%", n), Icon: "󰕾", Group: "audio",
			Cmd: portable.Volume(n),
		})
	}
	return append(acts, muteAction(), unmuteAction())
}

func muteAction() action.Action {
	return action.Action{ID: "vol:mute", Name: "Mute", Icon: "󰝟", Group: "audio", Cmd: portable.Mute(true)}
}

func unmuteAction() action.Action {
	return action.Action{ID: "vol:unmute", Name: "Unmute", Icon: "󰕾", Group: "audio", Cmd: portable.Mute(false)}
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
	switch sign, n, ok := level(q); {
	case ok && sign == "+":
		return []action.Action{{ID: "bright:up", Name: fmt.Sprintf("Brightness Up %d%%", n), Icon: "󰃠", Group: "display",
			Cmd: portable.Brightness(fmt.Sprintf("+%d", n))}}
	case ok && sign == "-":
		return []action.Action{{ID: "bright:down", Name: fmt.Sprintf("Brightness Down %d%%", n), Icon: "󰃞", Group: "display",
			Cmd: portable.Brightness(fmt.Sprintf("-%d", n))}}
	case ok && n >= 1:
		return []action.Action{{
			ID: "bright:set", Name: fmt.Sprintf("Brightness %d%%", n), Icon: "󰃟", Group: "display",
			Cmd: portable.Brightness(strconv.Itoa(n)),
		}}
	}
	var acts []action.Action
	for _, n := range []int{10, 30, 60, 100} {
		acts = append(acts, action.Action{
			ID: fmt.Sprintf("bright:%d", n), Name: fmt.Sprintf("Brightness %d%%", n), Icon: "󰃟", Group: "display",
			Cmd: portable.Brightness(strconv.Itoa(n)),
		})
	}
	return acts
}

// level reads "40", "40%", "+10", "-5", "up", "down" (a step of 5).
func level(q string) (sign string, n int, ok bool) {
	q = strings.TrimSuffix(strings.TrimSpace(q), "%")
	switch q {
	case "up":
		return "+", 5, true
	case "down":
		return "-", 5, true
	}
	if strings.HasPrefix(q, "+") || strings.HasPrefix(q, "-") {
		sign, q = q[:1], q[1:]
	}
	n, err := strconv.Atoi(q)
	if err != nil || n < 0 || n > 100 {
		return "", 0, false
	}
	return sign, n, true
}
