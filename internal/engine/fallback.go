package engine

import (
	"net/url"
	"os/exec"
	"regexp"
	"strings"

	"github.com/tuklu/nZlauncher/internal/action"
	"github.com/tuklu/nZlauncher/internal/shell"
)

// An unmatched query is never a dead end: it becomes an intent.
func fallbacks(raw string) []action.Action {
	q := strings.TrimSpace(raw)
	if q == "" {
		return nil
	}

	var fb []action.Action

	if result, ok := evalExpr(q); ok {
		fb = append(fb, action.Action{
			ID: "fallback:calc", Group: "fallback", Icon: "󰃬",
			Name: "= " + result + "   (enter to copy)",
			Cmd:  "wl-copy -n " + shell.Quote(result),
		})
	}

	if u, ok := urlAction(q); ok {
		fb = append(fb, u)
	} else if first := strings.Fields(q); len(first) > 0 {
		if _, err := exec.LookPath(first[0]); err == nil {
			script := q + `; printf '\n[done] press any key'; read -n1 -s`
			fb = append(fb, action.Action{
				ID: "fallback:run", Group: "fallback", Icon: "",
				Name: "Run: " + q,
				Cmd:  "launch-tui bash -c " + shell.Quote(script),
			})
		}
	}

	return append(fb, searchAction(q))
}

func searchAction(q string) action.Action {
	return action.Action{
		ID: "fallback:search", Group: "fallback", Icon: "󰍉",
		Name: "Search web: " + q,
		Cmd:  "uwsm-app -- xdg-open " + shell.Quote("https://www.google.com/search?q="+url.QueryEscape(q)),
	}
}

func urlAction(q string) (action.Action, bool) {
	if !isURL(q) {
		return action.Action{}, false
	}
	u := q
	if !strings.HasPrefix(u, "http") {
		u = "https://" + u
	}
	return action.Action{
		ID: "fallback:url", Group: "fallback", Icon: "󰖟",
		Name: "Open " + q,
		Cmd:  "uwsm-app -- xdg-open " + shell.Quote(u),
	}, true
}

var urlPattern = regexp.MustCompile(`^(https?://\S+|[a-zA-Z0-9-]+(\.[a-zA-Z0-9-]+)+(/\S*)?)$`)

func isURL(q string) bool {
	return urlPattern.MatchString(q)
}
