// Package learn turns the launch event log into frecency and context signals.
package learn

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/tuklu/nZlauncher/internal/config"
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
	return config.DataDir()
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
	events, _ := EventsSince(0)
	return events
}

// EventsSince reads the complete lines of the log from offset on and returns
// them with the offset just past the last complete line. A trailing line
// without its newline is left for the next call: a launch may be mid-append.
func EventsSince(offset int64) ([]Event, int64) {
	f, err := os.Open(eventsPath())
	if err != nil {
		return nil, 0
	}
	defer f.Close()
	if _, err := f.Seek(offset, io.SeekStart); err != nil {
		return nil, offset
	}
	data, err := io.ReadAll(f)
	if err != nil {
		return nil, offset
	}
	end := bytes.LastIndexByte(data, '\n')
	if end < 0 {
		return nil, offset
	}

	var events []Event
	for line := range bytes.SplitSeq(data[:end], []byte{'\n'}) {
		var e Event
		if json.Unmarshal(line, &e) == nil && e.ID != "" {
			events = append(events, e)
		}
	}
	return events, offset + int64(end) + 1
}

// EventsPath and FrecencyPath are exposed so callers can tell when they change.
func EventsPath() string   { return eventsPath() }
func FrecencyPath() string { return frecencyPath() }

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
