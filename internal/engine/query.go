// Package engine ranks every action source against a query.
package engine

import (
	"os"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/tuklu/nZlauncher/internal/action"
	"github.com/tuklu/nZlauncher/internal/answer"
	"github.com/tuklu/nZlauncher/internal/config"
	"github.com/tuklu/nZlauncher/internal/files"
	"github.com/tuklu/nZlauncher/internal/intent"
	"github.com/tuklu/nZlauncher/internal/learn"
	"github.com/tuklu/nZlauncher/internal/providers"
	"github.com/tuklu/nZlauncher/internal/shell"
)

const topCount = 8

var groupNames = map[string]bool{
	"power": true, "toggle": true, "capture": true, "info": true, "app": true,
}

// Query is the engine: every frontend consumes this one function. The window
// the user came from is read from NZ_FOCUSED, set by the bar for its face.
func Query(raw string) ([]action.Action, error) {
	return QueryFor(raw, os.Getenv("NZ_FOCUSED"))
}

// QueryFor is Query with the focused window given explicitly, for a daemon
// that answers many clients.
func QueryFor(raw, focusedClass string) ([]action.Action, error) {
	start := time.Now()
	snap, err := cache.snapshot(start)
	if err != nil {
		return nil, err
	}
	actions, stats := snap.actions, snap.stats
	ctx := queryContext{
		model:    snap.model,
		now:      start,
		ssid:     snap.ssid,
		settings: snap.settings,
		focused:  focusedClass,
	}
	now := ctx.now.Unix()
	q := strings.ToLower(strings.TrimSpace(raw))

	var ranked []action.Action
	switch {
	case q == "":
		ranked = topItems(append(actions, cache.windowList()...), stats, ctx)
	case q == "apps":
		ranked = byGroup(actions, "app", stats, now)
	case groupNames[q]:
		ranked = byGroup(actions, q, stats, now)
	default:
		ranked = answers(raw, start)
		if fq := intent.Files(strings.TrimSpace(raw), start); fq.Sure && providers.On("files") && !filePrefixed(q) {
			// an app called like the query still comes first: "docs" may be one
			pool := append(actions, cache.windowList()...)
			ranked = append(ranked, namePrefixed(rank(pool, stats, ctx, q), q)...)
			found := providers.FilterFiles(fq.Filter)
			if len(found) == 0 {
				found = []action.Action{providers.NoFiles(fq.Filter)}
			}
			ranked = append(ranked, found...)
			ranked = append(ranked, fallbacks(raw, ctx.settings)...)
		} else if nl := nlIntent(actions, stats, ctx, q); len(nl) > 0 {
			// a sentence that was understood is not a shell command too
			ranked = append(append(ranked, nl...), withoutRun(fallbacks(raw, ctx.settings))...)
		} else if provided, ok := providers.ByPrefix(strings.TrimSpace(raw)); ok {
			ranked = append(append(ranked, provided...), fallbacks(raw, ctx.settings)...)
		} else {
			pool := append(actions, cache.windowList()...)
			matched := withFiles(openOnce(rank(pool, stats, ctx, q)), q)
			ranked = append(append(ranked, matched...), fallbacks(raw, ctx.settings)...)
		}
	}

	return ranked, nil
}

func frecencySorted(actions []action.Action, stats map[string]learn.Stat, now int64) []action.Action {
	sort.SliceStable(actions, func(i, j int) bool {
		si, sj := learn.Score(stats[actions[i].ID], now), learn.Score(stats[actions[j].ID], now)
		if si != sj {
			return si > sj
		}
		return actions[i].Name < actions[j].Name
	})
	return actions
}

type queryContext struct {
	settings config.Settings
	model    *learn.Model
	now      time.Time
	ssid     string
	focused  string
}

func (c queryContext) bonus(id string) float64 {
	return c.model.Bonus(id, c.now, c.ssid, c.focused)
}

// First view: only what the user actually uses, reordered by current context
// (hour of day, what was just launched, network, focused app); pad out with
// defaults on a fresh install so the bar never opens empty.
func topItems(actions []action.Action, stats map[string]learn.Stat, ctx queryContext) []action.Action {
	now := ctx.now.Unix()
	sort.SliceStable(actions, func(i, j int) bool {
		si := learn.Score(stats[actions[i].ID], now) + ctx.bonus(actions[i].ID)
		sj := learn.Score(stats[actions[j].ID], now) + ctx.bonus(actions[j].ID)
		if si != sj {
			return si > sj
		}
		return actions[i].Name < actions[j].Name
	})

	top := make([]action.Action, 0, topCount)
	for _, a := range actions {
		if stats[a.ID].Hits > 0 {
			top = append(top, a)
			if len(top) == topCount {
				return top
			}
		}
	}
	for _, a := range actions {
		if stats[a.ID].Hits == 0 {
			top = append(top, a)
			if len(top) == topCount {
				break
			}
		}
	}
	return top
}

func byGroup(actions []action.Action, group string, stats map[string]learn.Stat, now int64) []action.Action {
	filtered := []action.Action{}
	for _, a := range actions {
		if a.Group == group {
			filtered = append(filtered, a)
		}
	}
	return frecencySorted(filtered, stats, now)
}

func rank(actions []action.Action, stats map[string]learn.Stat, ctx queryContext, q string) []action.Action {
	now := ctx.now.Unix()
	tokens := strings.Fields(q)
	type scored struct {
		action action.Action
		total  int
	}

	aliases := ctx.model.AliasBonuses(q)
	var hits []scored
	for _, a := range actions {
		m := matchScore(a, tokens)
		alias := int(aliases[a.ID])
		// a learned alias can surface an item the matcher rejects ("brw" → Chromium)
		if m == 0 && alias == 0 {
			continue
		}
		boost := int(learn.Score(stats[a.ID], now))
		if boost > frecencyCap {
			boost = frecencyCap
		}
		ctxBonus := int(ctx.bonus(a.ID))
		if ctxBonus > contextCap {
			ctxBonus = contextCap
		}
		hits = append(hits, scored{a, m + alias + boost + ctxBonus})
	}

	sort.SliceStable(hits, func(i, j int) bool {
		if hits[i].total != hits[j].total {
			return hits[i].total > hits[j].total
		}
		return hits[i].action.Name < hits[j].action.Name
	})

	ranked := make([]action.Action, len(hits))
	for i, h := range hits {
		ranked[i] = h.action
	}
	return ranked
}

// answers are computed results for the top of the list: conversions, the
// time somewhere, date math. Enter copies the value.
func answers(raw string, now time.Time) []action.Action {
	icons := map[string]string{"units": "󰟢", "currency": "󰄉", "time": "󰥔", "dates": "󰃭"}
	var acts []action.Action
	for _, a := range answer.For(raw, now, providers.On) {
		acts = append(acts, action.Action{
			ID: "answer:" + a.Kind, Group: "answer", Icon: icons[a.Kind],
			Name: "= " + a.Name, Hint: a.Hint,
			Cmd: "wl-copy -n " + shell.Quote(a.Value),
		})
	}
	return acts
}

// Files in a plain search: a strong file match (a name or a word in it
// starting with what was typed, or a file the user opens) joins the
// results without a prefix. Strong app and command matches keep the first
// places, at most two of them, so a file shows first when nothing else
// fits and second or third when something does.
const (
	filesInSearch   = 2
	filesAfterApps  = 2
	filesMinLetters = 3
)

func withFiles(ranked []action.Action, q string) []action.Action {
	if !providers.On("files") || !files.Ready() {
		return ranked
	}
	// every word needs three letters for the index's trigram lists: a
	// shorter one would mean reading every name, a cost plain typing is not
	// worth (the file prefix still does it)
	tokens := strings.Fields(q)
	for _, t := range tokens {
		if len(t) < filesMinLetters {
			return ranked
		}
	}
	hits, _ := files.Search(files.Filter{Terms: tokens, Limit: 6})
	var strong []files.Hit
	for _, h := range hits {
		if h.Strong && !h.Buried && len(strong) < filesInSearch {
			strong = append(strong, h)
		}
	}
	if len(strong) == 0 {
		return ranked
	}
	at := 0
	for at < len(ranked) && at < filesAfterApps && matchScore(ranked[at], tokens) >= scoreWordPrefix*len(tokens) {
		at++
	}
	return slices.Concat(ranked[:at], providers.FileActions(strong), ranked[at:])
}

func withoutRun(fb []action.Action) []action.Action {
	var out []action.Action
	for _, a := range fb {
		if a.ID != "fallback:run" {
			out = append(out, a)
		}
	}
	return out
}

// openOnce puts an app's open window right above the app: picking the app
// you already have open switches to it, and opening another is the next
// row down. Windows and apps meet on the window class.
func openOnce(ranked []action.Action) []action.Action {
	out := make([]action.Action, 0, len(ranked))
	moved := map[int]bool{}
	for i, a := range ranked {
		if moved[i] {
			continue
		}
		if a.Group == "app" && a.Class != "" {
			for j := i + 1; j < len(ranked); j++ {
				if w := ranked[j]; w.Group == "window" && !moved[j] && strings.EqualFold(w.Class, a.Class) {
					out = append(out, w)
					moved[j] = true
				}
			}
		}
		out = append(out, a)
	}
	return out
}

// "f pdfs" goes to the file provider, which reads the rest itself; with the
// whole query here the "f" would become a name term.
func filePrefixed(q string) bool {
	_, isF := providers.PrefixArg(q, "f")
	_, isFile := providers.PrefixArg(q, "file")
	return isF || isFile
}

// namePrefixed keeps the ranked actions whose name starts with q.
func namePrefixed(ranked []action.Action, q string) []action.Action {
	var res []action.Action
	for _, a := range ranked {
		if strings.HasPrefix(strings.ToLower(a.Name), q) {
			res = append(res, a)
		}
	}
	return res
}

// Warm loads every cached source now, so a daemon's first answer is as fast
// as the rest.
func Warm() {
	cache.snapshot(time.Now())
	cache.windowList()
}
