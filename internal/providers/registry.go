// Package providers turns system state into actions: applications, windows,
// devices, files, and anything in ~/.config/nzlauncher/providers.d.
package providers

import (
	"strings"

	"github.com/tuklu/nZlauncher/internal/action"
)

// "kill" → ("", true); "kill fire" → ("fire", true); "killer" → ("", false)
func PrefixArg(q, word string) (string, bool) {
	if q == word {
		return "", true
	}
	if rest, ok := strings.CutPrefix(q, word+" "); ok {
		return strings.TrimSpace(rest), true
	}
	return "", false
}

var prefixed = []struct {
	words []string
	fn    func(string) []action.Action
}{
	{[]string{"kill"}, Kill},
	{[]string{"wallpaper", "wp"}, Wallpaper},
	{[]string{"bluetooth", "bt"}, Bluetooth},
	{[]string{"audio", "sound", "output"}, Audio},
	{[]string{"wifi", "net", "network"}, Wifi},
	{[]string{"volume", "vol"}, Volume},
	{[]string{"workspace", "ws"}, Workspace},
	{[]string{"brightness", "bright"}, Brightness},
	{[]string{"file", "f"}, Files},
	{[]string{"monitor", "display", "mon"}, Monitor},
	{[]string{"clipboard", "clip"}, Clipboard},
	{[]string{"emoji"}, Emoji},
	{[]string{"keys", "binds", "shortcuts"}, Keys},
	{[]string{"transcode", "convert"}, Transcode},
}

// ByPrefix routes a query whose first word names a provider.
// q keeps the caller's case: providers that match text lowercase it
// themselves, ones that need raw input (file paths) get it intact.
func ByPrefix(q string) ([]action.Action, bool) {
	lower := strings.ToLower(q)
	for _, p := range prefixed {
		for _, w := range p.words {
			if _, ok := PrefixArg(lower, w); ok {
				return p.fn(strings.TrimSpace(q[min(len(w), len(q)):])), true
			}
		}
	}
	word, rest, _ := strings.Cut(lower, " ")
	if acts, ok := Plugin(word, strings.TrimSpace(rest)); ok {
		return acts, true
	}
	return nil, false
}
