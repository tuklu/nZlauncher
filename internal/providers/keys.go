package providers

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/tuklu/nZlauncher/internal/action"
	"github.com/tuklu/nZlauncher/internal/hypr"
	"github.com/tuklu/nZlauncher/internal/shell"
)

// keys / binds — every Hyprland binding, searchable; picking one executes it.
func Keys(q string) []action.Action {
	if !On("keys") {
		return nil
	}
	q = strings.ToLower(q)
	out, err := hypr.JSON("binds")
	if err != nil {
		return nil
	}
	var binds []struct {
		Modmask     int    `json:"modmask"`
		Key         string `json:"key"`
		Description string `json:"description"`
		Dispatcher  string `json:"dispatcher"`
		Arg         string `json:"arg"`
		Submap      string `json:"submap"`
	}
	if json.Unmarshal(out, &binds) != nil {
		return nil
	}

	luaBinds := scanLuaBinds()
	luaSeen := map[string]int{}

	var acts []action.Action
	for _, b := range binds {
		if b.Submap != "" || b.Key == "" {
			continue
		}

		var mods []string
		for _, m := range modifiers {
			if b.Modmask&m.bit != 0 {
				mods = append(mods, m.names[0])
			}
		}
		combo := b.Key
		if len(mods) > 0 {
			combo = strings.Join(mods, "+") + " + " + b.Key
		}
		pretty := prettyCombo(b.Modmask, b.Key)

		what := b.Description
		cmd := "true"
		switch {
		case !isLuaBind(b.Dispatcher):
			if what == "" {
				what = strings.TrimSpace(b.Dispatcher + " " + b.Arg)
			}
			if b.Dispatcher != "" {
				cmd = "hyprctl dispatch " + b.Dispatcher + " " + shell.Quote(b.Arg)
			}
		default:
			// __lua callbacks are opaque to hyprctl; recover them from the config source
			// a combo bound twice shows up twice, in config order
			id := bindKey(b.Modmask, b.Key)
			nth := luaSeen[id]
			luaSeen[id]++
			if nth >= len(luaBinds[id]) {
				if what == "" {
					what = "(lua action)"
				}
				break
			}
			lb := luaBinds[id][nth]
			if what == "" {
				what = lb.label
			}
			cmd = lb.cmd
		}
		name := humanize(what)
		// the combo is searchable both ways it is written
		haystack := strings.ToLower(name + " " + pretty + " " + combo + " " + what)
		if q != "" && !strings.Contains(haystack, q) {
			continue
		}

		acts = append(acts, action.Action{
			ID: "key:" + combo, Name: name, Icon: "󰌌", Group: "keys", Cmd: cmd,
			Hint: pretty, Keywords: []string{combo, what},
		})
	}
	return acts
}

var modifiers = []struct {
	bit   int
	names []string
}{
	{64, []string{"SUPER", "WIN", "LOGO", "MOD4", "META"}},
	{8, []string{"ALT", "MOD1"}},
	{4, []string{"CTRL", "CONTROL"}},
	{1, []string{"SHIFT"}},
}

func bindKey(modmask int, key string) string {
	return fmt.Sprintf("%d|%s", modmask, strings.ToLower(key))
}

type luaBind struct {
	label string
	cmd   string
}

// Lua config files are scanned for hl.bind("COMBO", EXPR, OPTS) calls whose
// combo is a plain string literal. Binds built in loops ("SUPER + code:" .. i)
// can't be resolved statically and stay view-only.
func scanLuaBinds() map[string][]luaBind {
	dir := os.Getenv("XDG_CONFIG_HOME")
	if dir == "" {
		dir = filepath.Join(os.Getenv("HOME"), ".config")
	}
	files, _ := filepath.Glob(filepath.Join(dir, "hypr", "*.lua"))

	found := map[string][]luaBind{}
	for _, path := range files {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		src := string(data)
		requires := strings.Join(requireLines.FindAllString(src, -1), "; ")

		for _, call := range findCalls(src, "hl.bind(") {
			if len(call) < 2 {
				continue
			}
			combo, ok := luaStringLiteral(call[0])
			if !ok {
				continue
			}
			mask, key, ok := parseCombo(combo)
			if !ok {
				continue
			}
			id := bindKey(mask, key)
			lb := describeLuaBind(call[1], requires)
			if len(call) > 2 {
				if m := descOption.FindStringSubmatch(call[2]); m != nil {
					lb.label = m[2]
				}
			}
			found[id] = append(found[id], lb)
		}
	}
	return found
}

var (
	requireLines = regexp.MustCompile(`(?m)^local\s+\w+\s*=\s*require\s*\(?\s*["'][^"']+["']\s*\)?`)
	descOption   = regexp.MustCompile(`desc(ription)?\s*=\s*["']([^"']*)["']`)
	execCmdCall  = regexp.MustCompile(`^hl\.dsp\.exec_cmd\(\s*(.+)\s*\)$`)
	inlineFunc   = regexp.MustCompile(`(?s)^function\s*\(\s*\)\s*(.*?)\s*end$`)
	whitespace   = regexp.MustCompile(`\s+`)
)

// exec_cmd binds run their shell command directly; anything else is
// evaluated inside Hyprland, with the file's require() locals in scope.
func describeLuaBind(expr, requires string) luaBind {
	expr = strings.TrimSpace(expr)

	if m := execCmdCall.FindStringSubmatch(expr); m != nil {
		if shell, ok := luaStringLiteral(m[1]); ok {
			return luaBind{label: shell, cmd: shell}
		}
	}

	label := whitespace.ReplaceAllString(expr, " ")
	if m := inlineFunc.FindStringSubmatch(label); m != nil {
		label = m[1]
	}
	label = strings.TrimPrefix(label, "hl.dsp.")

	lua := "local f = " + expr + "; if type(f) == 'function' then f() else hl.dispatch(f) end"
	if requires != "" {
		lua = requires + "; " + lua
	}
	cmd := "hyprctl eval " + shell.Quote(lua) +
		" >/dev/null || notify-send 'nZlauncher' " + shell.Quote("Keybind failed: "+label)
	return luaBind{label: label, cmd: cmd}
}

// "SUPER + SHIFT + Q" → (65, "Q")
func parseCombo(combo string) (int, string, bool) {
	mask := 0
	key := ""
	for _, part := range strings.Split(combo, "+") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if bit := modifierBit(part); bit != 0 {
			mask |= bit
			continue
		}
		if key != "" {
			return 0, "", false
		}
		key = part
	}
	return mask, key, key != ""
}

func modifierBit(name string) int {
	name = strings.ToUpper(name)
	for _, m := range modifiers {
		for _, n := range m.names {
			if n == name {
				return m.bit
			}
		}
	}
	return 0
}

// Accepts a single quoted Lua string with no concatenation.
func luaStringLiteral(s string) (string, bool) {
	s = strings.TrimSpace(s)
	if len(s) < 2 || (s[0] != '"' && s[0] != '\'') || s[len(s)-1] != s[0] {
		return "", false
	}
	quote := s[0]
	var b strings.Builder
	for i := 1; i < len(s)-1; i++ {
		c := s[i]
		if c == quote {
			return "", false // closed early: "a" .. "b"
		}
		if c == '\\' && i+1 < len(s)-1 {
			i++
			switch s[i] {
			case 'n':
				b.WriteByte('\n')
			case 't':
				b.WriteByte('\t')
			default:
				b.WriteByte(s[i])
			}
			continue
		}
		b.WriteByte(c)
	}
	return b.String(), true
}

// findCalls returns the top-level arguments of every `prefix(...)` call in
// src, honouring nested brackets, strings, and line comments.
func findCalls(src, prefix string) [][]string {
	var calls [][]string
	for i := 0; ; {
		at := strings.Index(src[i:], prefix)
		if at < 0 {
			return calls
		}
		start := i + at
		if lineCommented(src, start) {
			i = start + len(prefix)
			continue
		}
		args, end := splitArgs(src, start+len(prefix))
		if end < 0 {
			return calls
		}
		calls = append(calls, args)
		i = end
	}
}

func lineCommented(src string, pos int) bool {
	lineStart := strings.LastIndexByte(src[:pos], '\n') + 1
	return strings.Contains(src[lineStart:pos], "--")
}

// splitArgs scans from just after an opening paren to its match.
// Returns the arguments and the index after the closing paren, or -1.
func splitArgs(src string, pos int) ([]string, int) {
	var args []string
	depth := 0
	argStart := pos
	for i := pos; i < len(src); i++ {
		switch c := src[i]; c {
		case '"', '\'':
			for i++; i < len(src) && src[i] != c; i++ {
				if src[i] == '\\' {
					i++
				}
			}
		case '-':
			if i+1 < len(src) && src[i+1] == '-' {
				for i < len(src) && src[i] != '\n' {
					i++
				}
			}
		case '(', '{', '[':
			depth++
		case ')', '}', ']':
			if depth == 0 {
				args = append(args, strings.TrimSpace(src[argStart:i]))
				return args, i + 1
			}
			depth--
		case ',':
			if depth == 0 {
				args = append(args, strings.TrimSpace(src[argStart:i]))
				argStart = i + 1
			}
		default:
			if !isIdentByte(c) || (i > 0 && isIdentByte(src[i-1])) {
				continue
			}
			wordEnd := i
			for wordEnd < len(src) && isIdentByte(src[wordEnd]) {
				wordEnd++
			}
			switch src[i:wordEnd] {
			case "function", "if", "do", "repeat":
				depth++
			case "end", "until":
				depth--
			}
			i = wordEnd - 1
		}
	}
	return nil, -1
}

func isIdentByte(c byte) bool {
	return c == '_' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9'
}

var modifierLabels = []struct {
	bit   int
	label string
}{{64, "Super"}, {4, "Ctrl"}, {8, "Alt"}, {1, "Shift"}}

// Names for keys people do not know by their keysym.
var keyLabels = map[string]string{
	"xf86audioraisevolume": "Volume Up", "xf86audiolowervolume": "Volume Down",
	"xf86audiomute": "Mute", "xf86audiomicmute": "Mic Mute",
	"xf86audioplay": "Play", "xf86audiopause": "Pause", "xf86audiostop": "Stop",
	"xf86audionext": "Next Track", "xf86audioprev": "Previous Track",
	"xf86monbrightnessup": "Brightness Up", "xf86monbrightnessdown": "Brightness Down",
	"xf86kbdbrightnessup": "Keyboard Light Up", "xf86kbdbrightnessdown": "Keyboard Light Down",
	"xf86kbdlightonoff":  "Keyboard Light",
	"xf86touchpadtoggle": "Touchpad", "xf86touchpadon": "Touchpad On", "xf86touchpadoff": "Touchpad Off",
	"xf86poweroff": "Power", "xf86calculator": "Calculator", "xf86search": "Search",
	"return": "Enter", "escape": "Esc", "space": "Space", "tab": "Tab",
	"backspace": "Backspace", "delete": "Delete", "print": "Print",
	"left": "←", "right": "→", "up": "↑", "down": "↓",
	"period": ".", "comma": ",", "slash": "/", "minus": "-", "equal": "=",
	"bracketleft": "[", "bracketright": "]", "semicolon": ";", "apostrophe": "'", "grave": "`",
	"mouse_down": "Scroll Down", "mouse_up": "Scroll Up", "mouse_left": "Scroll Left", "mouse_right": "Scroll Right",
	"mouse:272": "Left Click", "mouse:273": "Right Click", "mouse:274": "Middle Click",
	// keycodes some layouts bind by: the number row and a few symbols
	"code:10": "1", "code:11": "2", "code:12": "3", "code:13": "4", "code:14": "5",
	"code:15": "6", "code:16": "7", "code:17": "8", "code:18": "9", "code:19": "0",
	"code:20": "-", "code:21": "=", "code:59": ",", "code:60": ".", "code:61": "/",
}

// "Super Shift B", "Volume Up", "Ctrl Alt ←"
func prettyCombo(modmask int, key string) string {
	var parts []string
	for _, m := range modifierLabels {
		if modmask&m.bit != 0 {
			parts = append(parts, m.label)
		}
	}
	label, ok := keyLabels[strings.ToLower(key)]
	switch {
	case ok:
	case strings.HasPrefix(key, "XF86"):
		label = splitCamel(strings.TrimPrefix(key, "XF86"))
	case len(key) == 1:
		label = strings.ToUpper(key)
	default:
		label = strings.ToUpper(key[:1]) + strings.ToLower(key[1:])
	}
	return strings.Join(append(parts, label), " ")
}

func splitCamel(s string) string {
	var b strings.Builder
	for i, r := range s {
		if i > 0 && r >= 'A' && r <= 'Z' {
			b.WriteByte(' ')
		}
		b.WriteRune(r)
	}
	return b.String()
}

var luaCall = regexp.MustCompile(`^(?:\w+\.)*(\w+)\((.*)\)$`)

// Readable text for a bind without a description: Lua calls become words
// ("actions.toggle_touchpad("on")" is "Toggle touchpad on"); shell commands
// stay as they are, they already read as what they do.
func humanize(what string) string {
	m := luaCall.FindStringSubmatch(strings.TrimSpace(what))
	if m == nil {
		return what
	}
	words := strings.ReplaceAll(m[1], "_", " ")
	args := strings.Trim(strings.NewReplacer(`"`, "", "'", "", "{", "", "}", "").Replace(m[2]), " ")
	if args != "" {
		words += " " + args
	}
	return strings.ToUpper(words[:1]) + words[1:]
}

// Lua-configured Hyprland reports a bind's action as "__lua" (0.56), or as
// "HL.Dispatcher(name)" or "function: 0x..." (0.57); the real action is only
// in the config source.
func isLuaBind(dispatcher string) bool {
	return dispatcher == "__lua" ||
		strings.HasPrefix(dispatcher, "HL.Dispatcher(") ||
		strings.HasPrefix(dispatcher, "function:")
}
