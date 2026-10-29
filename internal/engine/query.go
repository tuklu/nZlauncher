// Package engine ranks every action source against a query.
package engine

import (
	"os"
	"sort"
	"strings"
	"time"

	"github.com/tuklu/nZlauncher/internal/action"
	"github.com/tuklu/nZlauncher/internal/config"
	"github.com/tuklu/nZlauncher/internal/learn"
	"github.com/tuklu/nZlauncher/internal/providers"
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
		if nl := nlIntent(actions, stats, ctx, q); len(nl) > 0 {
			ranked = append(nl, fallbacks(raw, ctx.settings)...)
		} else if provided, ok := providers.ByPrefix(strings.TrimSpace(raw)); ok {
			ranked = append(provided, fallbacks(raw, ctx.settings)...)
		} else {
			pool := append(actions, cache.windowList()...)
			ranked = append(rank(pool, stats, ctx, q), fallbacks(raw, ctx.settings)...)
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

// Warm loads every cached source now, so a daemon's first answer is as fast
// as the rest.
func Warm() {
	cache.snapshot(time.Now())
	cache.windowList()
}
