package engine

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/tuklu/nZlauncher/internal/action"
	"github.com/tuklu/nZlauncher/internal/config"
	"github.com/tuklu/nZlauncher/internal/icons"
	"github.com/tuklu/nZlauncher/internal/learn"
	"github.com/tuklu/nZlauncher/internal/providers"
	"github.com/tuklu/nZlauncher/internal/sysinfo"
)

// How long a looked-up SSID is trusted; spawning iw on every keystroke costs
// more than the rest of a query.
const ssidTTL = 10 * time.Second

// state keeps what a query needs between keystrokes of one process. Every
// query stats the source files (microseconds) and reloads only what changed.
type state struct {
	mu sync.Mutex

	catalogStamp string
	catalog      config.Catalog
	catalogErr   error

	appsStamp string
	apps      []action.Action

	eventsOffset  int64
	events        []learn.Event
	frecencyStamp string
	learnedValid  bool
	stats         map[string]learn.Stat
	model         *learn.Model

	ssid   string
	ssidAt time.Time
}

var cache state

// snapshot is one query's view: actions is a private copy, since ranking
// sorts and appends in place.
type snapshot struct {
	actions []action.Action
	stats   map[string]learn.Stat
	model   *learn.Model
	ssid    string
}

func (s *state) snapshot(now time.Time) (snapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.refreshCatalog()
	if s.catalogErr != nil {
		return snapshot{}, s.catalogErr
	}
	s.refreshApps()
	s.refreshLearned()
	if now.Sub(s.ssidAt) > ssidTTL {
		s.ssid, s.ssidAt = sysinfo.CurrentSSID(), now
	}

	actions := make([]action.Action, 0, len(s.catalog.Actions)+len(s.apps))
	actions = append(append(actions, s.catalog.Actions...), s.apps...)
	return snapshot{actions: actions, stats: s.stats, model: s.model, ssid: s.ssid}, nil
}

func (s *state) refreshCatalog() {
	stamp := stampOf(config.CatalogPath())
	if stamp == s.catalogStamp && s.catalogStamp != "" {
		return
	}
	s.catalog, s.catalogErr = config.Load()
	// stamp after loading: Load writes the default catalog on a first run
	s.catalogStamp = stampOf(config.CatalogPath())
}

// Apps depend on the .desktop directories and on the icon library they are
// decorated from.
func (s *state) refreshApps() {
	iconDir := filepath.Join(config.Dir(), "icons")
	stamp := s.catalog.Settings.IconLibrary + stampOf(append(providers.AppDirs(), iconDir)...)
	if stamp == s.appsStamp && s.apps != nil {
		return
	}
	lib := icons.Load(iconDir, s.catalog.Settings.IconLibrary)
	s.apps = providers.Apps(lib)
	s.appsStamp = s.catalog.Settings.IconLibrary + stampOf(append(providers.AppDirs(), iconDir)...)
}

// The event log is append-only: read the new tail, and rebuild the
// aggregates only when something arrived or the legacy baseline changed.
func (s *state) refreshLearned() {
	if info, err := os.Stat(learn.EventsPath()); err != nil || info.Size() < s.eventsOffset {
		s.events, s.eventsOffset, s.learnedValid = nil, 0, false
	}
	fresh, offset := learn.EventsSince(s.eventsOffset)
	s.eventsOffset = offset
	s.events = append(s.events, fresh...)

	frecency := stampOf(learn.FrecencyPath())
	if len(fresh) == 0 && frecency == s.frecencyStamp && s.learnedValid {
		return
	}
	s.stats = learn.Stats(s.events)
	s.model = learn.BuildModel(s.events)
	s.frecencyStamp, s.learnedValid = frecency, true
}

// stampOf changes whenever any of the paths is created, removed, renamed
// into, or rewritten. For a directory, adding or removing an entry counts.
func stampOf(paths ...string) string {
	var b strings.Builder
	for _, p := range paths {
		if info, err := os.Stat(p); err == nil {
			fmt.Fprintf(&b, "%d:%d;", info.ModTime().UnixNano(), info.Size())
		} else {
			b.WriteString("-;")
		}
	}
	return b.String()
}
