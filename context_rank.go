package main

import (
	"strings"
	"time"
)

// Learned context model, rebuilt from the event log on every query.
// All bonuses are gated by minimum sample counts so sparse data stays silent.
type model struct {
	total     map[string]int
	hours     map[string]*[24]int
	successor map[string]map[string]int
	ssid      map[string]map[string]int
	focus     map[string]map[string]int
	aliases   map[string]map[string]int // typed query → picked id → count
	last      *Event
}

const successorWindow = 900 // seconds: launches this close count as a sequence

func buildModel(events []Event) *model {
	m := &model{
		total:     map[string]int{},
		hours:     map[string]*[24]int{},
		successor: map[string]map[string]int{},
		ssid:      map[string]map[string]int{},
		focus:     map[string]map[string]int{},
		aliases:   map[string]map[string]int{},
	}

	var prev *Event
	for i := range events {
		e := &events[i]
		m.total[e.ID]++

		h := time.Unix(e.TS, 0).Hour()
		if m.hours[e.ID] == nil {
			m.hours[e.ID] = &[24]int{}
		}
		m.hours[e.ID][h]++

		if prev != nil && prev.ID != e.ID && e.TS-prev.TS < successorWindow {
			if m.successor[prev.ID] == nil {
				m.successor[prev.ID] = map[string]int{}
			}
			m.successor[prev.ID][e.ID]++
		}

		if e.SSID != "" {
			if m.ssid[e.ID] == nil {
				m.ssid[e.ID] = map[string]int{}
			}
			m.ssid[e.ID][e.SSID]++
		}
		if e.Focused != "" {
			if m.focus[e.ID] == nil {
				m.focus[e.ID] = map[string]int{}
			}
			m.focus[e.ID][e.Focused]++
		}
		if q := strings.ToLower(strings.TrimSpace(e.Query)); q != "" {
			if m.aliases[q] == nil {
				m.aliases[q] = map[string]int{}
			}
			m.aliases[q][e.ID]++
		}
		prev = e
	}

	if len(events) > 0 {
		m.last = &events[len(events)-1]
	}
	return m
}

func (m *model) bonus(id string, now time.Time, curSSID, curFocus string) float64 {
	b := 0.0
	total := m.total[id]

	// Hour affinity: how concentrated this item's launches are around now,
	// compared to a uniform spread (5 of 24 hours in the +/-2h window).
	if total >= 5 && m.hours[id] != nil {
		h := now.Hour()
		near := 0
		for d := -2; d <= 2; d++ {
			near += m.hours[id][(h+d+24)%24]
		}
		frac := float64(near) / float64(total)
		const uniform = 5.0 / 24.0
		if frac > uniform {
			b += 20 * (frac - uniform) / (1 - uniform)
		}
	}

	// Markov successor: you usually launch this right after the previous pick.
	if m.last != nil && m.last.ID != id && now.Unix()-m.last.TS < successorWindow {
		if succ := m.successor[m.last.ID]; succ != nil && succ[id] >= 2 {
			succTotal := 0
			for _, c := range succ {
				succTotal += c
			}
			b += 30 * float64(succ[id]) / float64(succTotal)
		}
	}

	// Environment affinity: same network, same focused app as past launches.
	if total >= 3 {
		if curSSID != "" && m.ssid[id] != nil {
			b += 10 * float64(m.ssid[id][curSSID]) / float64(total)
		}
		if curFocus != "" && m.focus[id] != nil {
			b += 15 * float64(m.focus[id][curFocus]) / float64(total)
		}
	}

	return b
}

// Learned aliases: this exact query (or one it prefixes) led to these picks
// before. Two confirmations required; exact matches can outrank anything.
func (m *model) aliasBonuses(q string) map[string]float64 {
	bonuses := map[string]float64{}
	if q == "" {
		return bonuses
	}

	apply := func(picks map[string]int, scale float64) {
		total := 0
		for _, c := range picks {
			total += c
		}
		for id, c := range picks {
			if c < 2 {
				continue
			}
			if v := scale * float64(c) / float64(total); v > bonuses[id] {
				bonuses[id] = v
			}
		}
	}

	if picks := m.aliases[q]; picks != nil {
		apply(picks, 150)
	}
	if len(q) >= 2 {
		for stored, picks := range m.aliases {
			if stored != q && strings.HasPrefix(stored, q) {
				apply(picks, 70)
			}
		}
	}
	return bonuses
}
