package main

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// Append-only launch log: the source of truth for all learning.
// frecency.json is kept read-only as the pre-log baseline.
type Event struct {
	ID      string `json:"id"`
	TS      int64  `json:"ts"`
	Query   string `json:"q,omitempty"`
	Battery bool   `json:"bat,omitempty"`
	SSID    string `json:"ssid,omitempty"`
	Focused string `json:"focus,omitempty"`
}

func eventsPath() string {
	return filepath.Join(os.Getenv("HOME"), ".local/share/neZha/events.jsonl")
}

func recordLaunch(id, query string) error {
	e := Event{
		ID:      id,
		TS:      time.Now().Unix(),
		Query:   query,
		Battery: onBattery(),
		SSID:    currentSSID(),
		Focused: os.Getenv("NZ_FOCUSED"),
	}
	data, err := json.Marshal(e)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(eventsPath()), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(eventsPath(), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(append(data, '\n'))
	return err
}

func loadEvents() []Event {
	f, err := os.Open(eventsPath())
	if err != nil {
		return nil
	}
	defer f.Close()

	var events []Event
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var e Event
		if json.Unmarshal(scanner.Bytes(), &e) == nil && e.ID != "" {
			events = append(events, e)
		}
	}
	return events
}

// Legacy aggregates as baseline, then replay the event log on top.
func statsFrom(events []Event) map[string]stat {
	stats := map[string]stat{}
	if data, err := os.ReadFile(frecencyPath()); err == nil {
		json.Unmarshal(data, &stats)
	}
	for _, e := range events {
		s := stats[e.ID]
		s.Hits++
		if e.TS > s.Last {
			s.Last = e.TS
		}
		stats[e.ID] = s
	}
	return stats
}

func loadStats() map[string]stat {
	return statsFrom(loadEvents())
}
