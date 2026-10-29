// Package wifi reads and drives Wi-Fi through whichever service runs it,
// over the system bus: iwd or NetworkManager. The state is cached and the
// service's change signals mark it stale, so a query never waits on iwctl or
// nmcli. The radio itself is rfkill's, the same for every backend.
package wifi

import (
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/godbus/dbus/v5"
	"github.com/tuklu/nZlauncher/internal/platform"
)

type Network struct {
	SSID      string
	Security  string // open, psk, 8021x, wep
	Signal    int    // percent
	Known     bool   // connects without asking for anything
	Connected bool
}

type State struct {
	Backend   string // iwd, networkmanager
	Interface string // wlan0
	Enabled   bool   // the radio is not blocked
	Networks  []Network
}

// Connected is the SSID in use, "" when none.
func (s State) Connected() string {
	for _, n := range s.Networks {
		if n.Connected {
			return n.SSID
		}
	}
	return ""
}

type backend struct {
	bus        string
	fetch      func(conn *dbus.Conn) (State, error)
	connect    func(conn *dbus.Conn, ssid string) error
	disconnect func(conn *dbus.Conn) error
	scan       func(conn *dbus.Conn)
}

var backends = map[string]backend{
	"iwd":            iwd,
	"networkmanager": networkManager,
}

var cache struct {
	sync.Mutex
	conn    *dbus.Conn
	name    string // backend the signals are watched for
	stale   bool
	state   State
	scanned time.Time
	signals chan *dbus.Signal
}

// ErrNoBackend: no supported Wi-Fi service runs.
var ErrNoBackend = errors.New("no Wi-Fi service found (iwd or NetworkManager)")

func current() (backend, string, bool) {
	name := platform.Current().Wifi
	b, ok := backends[name]
	return b, name, ok
}

// Current is the cached state, refetched only after the service signalled a
// change. Empty when no supported service runs.
func Current() State {
	b, name, ok := current()
	if !ok {
		return State{Enabled: radioEnabled()}
	}
	cache.Lock()
	defer cache.Unlock()
	if cache.conn == nil {
		conn, err := dbus.ConnectSystemBus()
		if err != nil {
			return State{}
		}
		cache.conn = conn
		cache.signals = make(chan *dbus.Signal, 128)
		conn.Signal(cache.signals)
		go func() {
			for sig := range cache.signals {
				cache.Lock()
				cache.stale = true
				cache.Unlock()
				settingsChanged(sig)
			}
		}()
	}
	if cache.name != name {
		watch(cache.conn, b.bus)
		cache.name, cache.stale = name, true
	}
	if cache.stale {
		state, err := b.fetch(cache.conn)
		if err != nil {
			return State{Backend: name, Enabled: radioEnabled()}
		}
		state.Backend = name
		sortNetworks(state.Networks)
		cache.state, cache.stale = state, false
	}
	s := cache.state
	s.Enabled = radioEnabled() // rfkill sends no bus signal; reading sysfs is cheap
	return s
}

func watch(conn *dbus.Conn, bus string) {
	conn.AddMatchSignal(dbus.WithMatchSender(bus))
	// the service restarting is a new owner of its name
	conn.AddMatchSignal(dbus.WithMatchInterface("org.freedesktop.DBus"), dbus.WithMatchMember("NameOwnerChanged"), dbus.WithMatchArg(0, bus))
}

// Scan asks the service for fresh results, at most every 30 seconds; they
// arrive through the signals. It does not wait.
func Scan() {
	b, _, ok := current()
	if !ok {
		return
	}
	cache.Lock()
	conn := cache.conn
	due := time.Since(cache.scanned) > 30*time.Second
	if due {
		cache.scanned = time.Now()
	}
	cache.Unlock()
	if conn != nil && due {
		go b.scan(conn)
	}
}

// Connect joins a network the service can join without asking: a known or
// an open one.
func Connect(ssid string) error {
	b, _, ok := current()
	if !ok {
		return ErrNoBackend
	}
	conn, err := dbus.ConnectSystemBus()
	if err != nil {
		return err
	}
	defer conn.Close()
	return b.connect(conn, ssid)
}

func Disconnect() error {
	b, _, ok := current()
	if !ok {
		return ErrNoBackend
	}
	conn, err := dbus.ConnectSystemBus()
	if err != nil {
		return err
	}
	defer conn.Close()
	return b.disconnect(conn)
}

// connected, then known, then by signal; one entry per SSID
func sortNetworks(ns []Network) {
	sort.SliceStable(ns, func(i, j int) bool {
		a, b := ns[i], ns[j]
		if a.Connected != b.Connected {
			return a.Connected
		}
		if a.Known != b.Known {
			return a.Known
		}
		return a.Signal > b.Signal
	})
}

// dedupe keeps the strongest entry per SSID; access points of one network
// show up once each.
func dedupe(ns []Network) []Network {
	best := map[string]int{}
	var out []Network
	for _, n := range ns {
		if n.SSID == "" {
			continue // hidden
		}
		if k, seen := best[n.SSID]; seen {
			if n.Signal > out[k].Signal {
				n.Known, n.Connected = n.Known || out[k].Known, n.Connected || out[k].Connected
				out[k] = n
			} else {
				out[k].Connected = out[k].Connected || n.Connected
			}
			continue
		}
		best[n.SSID] = len(out)
		out = append(out, n)
	}
	return out
}

func radioEnabled() bool {
	radios, _ := filepath.Glob("/sys/class/rfkill/rfkill*")
	found := false
	for _, r := range radios {
		if kind, _ := os.ReadFile(r + "/type"); strings.TrimSpace(string(kind)) != "wlan" {
			continue
		}
		found = true
		soft, _ := os.ReadFile(r + "/soft")
		hard, _ := os.ReadFile(r + "/hard")
		if strings.TrimSpace(string(soft)) == "0" && strings.TrimSpace(string(hard)) == "0" {
			return true
		}
	}
	return !found // no rfkill switch at all: nothing blocks it
}

// dBm to percent the way NetworkManager does: -100 is 0, -50 and up is 100.
func dbmPercent(dbm int) int {
	return min(max(2*(dbm+100), 0), 100)
}

func variantString(v dbus.Variant) string {
	s, _ := v.Value().(string)
	return s
}

func variantBool(v dbus.Variant) bool {
	b, _ := v.Value().(bool)
	return b
}
