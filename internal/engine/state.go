package engine

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/tuklu/nZlauncher/internal/action"
	"github.com/tuklu/nZlauncher/internal/config"
	"github.com/tuklu/nZlauncher/internal/hypr"
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

	settingsStamp string
	settings      config.Settings

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

	windows []action.Action
}

// Set by the Hyprland event watcher whenever the window list may differ from
// the cached one. Starts true so the first query fetches.
var (
	windowsStale    atomic.Bool
	watchingWindows atomic.Bool
	startWatch      sync.Once
)

// Events that change what the window provider shows: which windows exist,
// their titles, and where they are.
var windowEvents = map[string]bool{
	"openwindow": true, "closewindow": true, "movewindow": true, "movewindowv2": true,
	"windowtitle": true, "windowtitlev2": true,
}

// windowList is the open windows, refetched only after Hyprland reported a
// change. Without the event socket it fetches every time, as before.
func (s *state) windowList() []action.Action {
	startWatch.Do(func() {
		windowsStale.Store(true)
		watchingWindows.Store(true)
		go func() {
			hypr.Watch(func(e string) bool { return windowEvents[e] }, func() { windowsStale.Store(true) })
			watchingWindows.Store(false)
		}()
	})

	s.mu.Lock()
	defer s.mu.Unlock()
	// clear the flag before fetching, so an event during the fetch is not lost
	if windowsStale.Swap(false) || !watchingWindows.Load() || s.windows == nil {
		s.windows = providers.Windows()
	}
	return slices.Clone(s.windows)
}

var cache state

// snapshot is one query's view: actions is a private copy, since ranking
// sorts and appends in place.
type snapshot struct {
	actions  []action.Action
	stats    map[string]learn.Stat
	model    *learn.Model
	ssid     string
	settings config.Settings
}

func (s *state) snapshot(now time.Time) (snapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.refreshSettings()
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
	return snapshot{actions: actions, stats: s.stats, model: s.model, ssid: s.ssid, settings: s.settings}, nil
}

// The catalog stamp is part of it: a legacy [settings] block lives there.
func (s *state) refreshSettings() {
	stamp := stampOf(config.ConfigPath(), config.CatalogPath())
	if stamp == s.settingsStamp && s.settingsStamp != "" {
		return
	}
	// a broken config.toml keeps the last good settings
	if settings, err := config.LoadSettings(); err == nil || s.settingsStamp == "" {
		s.settings = settings
	}
	s.settingsStamp = stampOf(config.ConfigPath(), config.CatalogPath())
	settings := s.settings
	providers.Configure(settings.Provider)
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
	stamp := s.settings.IconLibrary + stampOf(append(providers.AppDirs(), iconDir)...)
	if stamp == s.appsStamp && s.apps != nil {
		return
	}
	lib := icons.Load(iconDir, s.settings.IconLibrary)
	s.apps = providers.Apps(lib)
	s.appsStamp = s.settings.IconLibrary + stampOf(append(providers.AppDirs(), iconDir)...)
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
