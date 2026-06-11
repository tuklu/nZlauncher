package main

import (
	"net/url"
	"os/exec"
	"regexp"
	"strings"
)

// An unmatched query is never a dead end: it becomes an intent.
func fallbacks(raw string) []Action {
	q := strings.TrimSpace(raw)
	if q == "" {
		return nil
	}

	var fb []Action

	if result, ok := evalExpr(q); ok {
		fb = append(fb, Action{
			ID: "fallback:calc", Group: "fallback", Icon: "󰃬",
			Name: "= " + result + "   (enter to copy)",
			Cmd:  "wl-copy -n " + shellQuote(result),
		})
	}

	if u, ok := urlAction(q); ok {
		fb = append(fb, u)
	} else if first := strings.Fields(q); len(first) > 0 {
		if _, err := exec.LookPath(first[0]); err == nil {
			script := q + `; printf '\n[done] press any key'; read -n1 -s`
			fb = append(fb, Action{
				ID: "fallback:run", Group: "fallback", Icon: "",
				Name: "Run: " + q,
				Cmd:  "launch-tui bash -c " + shellQuote(script),
			})
		}
	}

	return append(fb, searchAction(q))
}

func searchAction(q string) Action {
	return Action{
		ID: "fallback:search", Group: "fallback", Icon: "󰍉",
		Name: "Search web: " + q,
		Cmd:  "uwsm-app -- xdg-open " + shellQuote("https://www.google.com/search?q="+url.QueryEscape(q)),
	}
}

func urlAction(q string) (Action, bool) {
	if !isURL(q) {
		return Action{}, false
	}
	u := q
	if !strings.HasPrefix(u, "http") {
		u = "https://" + u
	}
	return Action{
		ID: "fallback:url", Group: "fallback", Icon: "󰖟",
		Name: "Open " + q,
		Cmd:  "uwsm-app -- xdg-open " + shellQuote(u),
	}, true
}

var urlPattern = regexp.MustCompile(`^(https?://\S+|[a-zA-Z0-9-]+(\.[a-zA-Z0-9-]+)+(/\S*)?)$`)

func isURL(q string) bool {
	return urlPattern.MatchString(q)
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
