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

		what := b.Description
		cmd := "true"
		switch {
		case b.Dispatcher != "__lua":
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
		name := fmt.Sprintf("%-28s %s", combo, what)

		if q != "" && !strings.Contains(strings.ToLower(name), q) {
			continue
		}

		acts = append(acts, action.Action{
			ID: "key:" + combo, Name: name, Icon: "󰌌", Group: "keys", Cmd: cmd,
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
