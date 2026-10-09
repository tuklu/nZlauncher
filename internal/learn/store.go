// Package learn turns the launch event log into frecency and context signals.
package learn

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/tuklu/nZlauncher/internal/sysinfo"
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

func dataDir() string {
	return filepath.Join(os.Getenv("HOME"), ".local/share/neZha")
}

func eventsPath() string {
	return filepath.Join(dataDir(), "events.jsonl")
}

func frecencyPath() string {
	return filepath.Join(dataDir(), "frecency.json")
}

// Record appends one launch to the event log, tagged with current context.
func Record(id, query string) error {
	e := Event{
		ID:      id,
		TS:      time.Now().Unix(),
		Query:   query,
		Battery: sysinfo.OnBattery(),
		SSID:    sysinfo.CurrentSSID(),
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

func LoadEvents() []Event {
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
func Stats(events []Event) map[string]Stat {
	stats := map[string]Stat{}
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
