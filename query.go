package main

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"time"
)

const topCount = 8

func gatherAll() ([]Action, error) {
	actions, err := loadCatalog(catalogPath())
	if err != nil {
		return nil, err
	}
	return append(actions, scanApps()...), nil
}

var groupNames = map[string]bool{
	"power": true, "toggle": true, "capture": true, "info": true, "app": true,
}

// "kill" → ("", true); "kill fire" → ("fire", true); "killer" → ("", false)
func prefixArg(q, word string) (string, bool) {
	if q == word {
		return "", true
	}
	if rest, ok := strings.CutPrefix(q, word+" "); ok {
		return strings.TrimSpace(rest), true
	}
	return "", false
}

var prefixProviders = []struct {
	words []string
	fn    func(string) []Action
}{
	{[]string{"kill"}, killActions},
	{[]string{"wallpaper", "wp"}, wallpaperActions},
	{[]string{"bluetooth", "bt"}, bluetoothActions},
	{[]string{"audio", "sound", "output"}, audioActions},
	{[]string{"wifi", "net", "network"}, wifiActions},
	{[]string{"volume", "vol"}, volumeActions},
	{[]string{"workspace", "ws"}, workspaceActions},
	{[]string{"brightness", "bright"}, brightnessActions},
	{[]string{"file", "f"}, fileActions},
	{[]string{"monitor", "display", "mon"}, monitorActions},
}

func fromProvider(q string) ([]Action, bool) {
	for _, p := range prefixProviders {
		for _, w := range p.words {
			if rest, ok := prefixArg(q, w); ok {
				return p.fn(rest), true
			}
		}
	}
	return nil, false
}

func runQuery(raw string) error {
	actions, err := gatherAll()
	if err != nil {
		return err
	}
	events := loadEvents()
	stats := statsFrom(events)
	ctx := queryContext{
		model:   buildModel(events),
		now:     time.Now(),
		ssid:    currentSSID(),
		focused: os.Getenv("NZ_FOCUSED"),
	}
	now := ctx.now.Unix()
	q := strings.ToLower(strings.TrimSpace(raw))

	var ranked []Action
	switch {
	case q == "":
		ranked = topItems(append(actions, windowActions()...), stats, ctx)
	case q == "apps":
		ranked = byGroup(actions, "app", stats, now)
	case groupNames[q]:
		ranked = byGroup(actions, q, stats, now)
	default:
		if nl := nlIntent(actions, stats, ctx, q); len(nl) > 0 {
			ranked = append(nl, fallbacks(raw)...)
		} else if provided, ok := fromProvider(q); ok {
			ranked = append(provided, fallbacks(raw)...)
		} else {
			pool := append(actions, windowActions()...)
			ranked = append(rank(pool, stats, ctx, q), fallbacks(raw)...)
		}
	}

	for _, a := range ranked {
		fmt.Printf("%s\t%s\t%s\n", a.ID, a.Cmd, a.display())
	}
	return nil
}

func frecencySorted(actions []Action, stats map[string]stat, now int64) []Action {
	sort.SliceStable(actions, func(i, j int) bool {
		si, sj := score(stats[actions[i].ID], now), score(stats[actions[j].ID], now)
		if si != sj {
			return si > sj
		}
		return actions[i].Name < actions[j].Name
	})
	return actions
}

type queryContext struct {
	model   *model
	now     time.Time
	ssid    string
	focused string
}

func (c queryContext) bonus(id string) float64 {
	return c.model.bonus(id, c.now, c.ssid, c.focused)
}

// First view: only what the user actually uses, reordered by current context
// (hour of day, what was just launched, network, focused app); pad out with
// defaults on a fresh install so the bar never opens empty.
func topItems(actions []Action, stats map[string]stat, ctx queryContext) []Action {
	now := ctx.now.Unix()
	sort.SliceStable(actions, func(i, j int) bool {
		si := score(stats[actions[i].ID], now) + ctx.bonus(actions[i].ID)
		sj := score(stats[actions[j].ID], now) + ctx.bonus(actions[j].ID)
		if si != sj {
			return si > sj
		}
		return actions[i].Name < actions[j].Name
	})

	top := make([]Action, 0, topCount)
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

func byGroup(actions []Action, group string, stats map[string]stat, now int64) []Action {
	filtered := []Action{}
	for _, a := range actions {
		if a.Group == group {
			filtered = append(filtered, a)
		}
	}
	return frecencySorted(filtered, stats, now)
}

func rank(actions []Action, stats map[string]stat, ctx queryContext, q string) []Action {
	now := ctx.now.Unix()
	tokens := strings.Fields(q)
	type scored struct {
		action Action
		total  int
	}

	aliases := ctx.model.aliasBonuses(q)
	var hits []scored
	for _, a := range actions {
		m := matchScore(a, tokens)
		alias := int(aliases[a.ID])
		// a learned alias can surface an item the matcher rejects ("brw" → Chromium)
		if m == 0 && alias == 0 {
			continue
		}
		boost := int(score(stats[a.ID], now))
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

	ranked := make([]Action, len(hits))
	for i, h := range hits {
		ranked[i] = h.action
	}
	return ranked
}
