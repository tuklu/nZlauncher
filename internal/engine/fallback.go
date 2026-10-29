package engine

import (
	"net/url"
	"regexp"
	"strings"

	"github.com/tuklu/nZlauncher/internal/action"
	"github.com/tuklu/nZlauncher/internal/config"
	"github.com/tuklu/nZlauncher/internal/portable"
	"github.com/tuklu/nZlauncher/internal/shell"
)

// An unmatched query is never a dead end: it becomes an intent.
func fallbacks(raw string, s config.Settings) []action.Action {
	q := strings.TrimSpace(raw)
	if q == "" {
		return nil
	}

	var fb []action.Action

	if result, ok := evalExpr(q); ok && s.Fallback("calc") {
		fb = append(fb, action.Action{
			ID: "fallback:calc", Group: "fallback", Icon: "󰃬",
			Name: "= " + result + "   (enter to copy)",
			Cmd:  "wl-copy -n " + shell.Quote(result),
		})
	}

	if u, ok := urlAction(q); ok {
		if s.Fallback("url") {
			fb = append(fb, u)
		}
	} else if first := strings.Fields(q); len(first) > 0 && s.Fallback("run") {
		script := q + `; printf '\n[done] press any key'; read -n1 -s`
		if term := portable.Terminal("bash -c " + shell.Quote(script)); term != "" && portable.Have(first[0]) {
			fb = append(fb, action.Action{
				ID: "fallback:run", Group: "fallback", Icon: "",
				Name: "Run: " + q,
				Cmd:  term,
			})
		}
	}

	if s.Fallback("search") {
		fb = append(fb, searchAction(q, s))
	}
	return fb
}

func searchAction(q string, s config.Settings) action.Action {
	return action.Action{
		ID: "fallback:search", Group: "fallback", Icon: "󰍉",
		Name: "Search web: " + q,
		Cmd:  portable.Open(strings.ReplaceAll(s.SearchURL(), "%s", url.QueryEscape(q))),
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
		Cmd:  portable.Open(u),
	}, true
}

var urlPattern = regexp.MustCompile(`^(https?://\S+|[a-zA-Z0-9-]+(\.[a-zA-Z0-9-]+)+(/\S*)?)$`)

func isURL(q string) bool {
	return urlPattern.MatchString(q)
}
