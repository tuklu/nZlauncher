package tweaks

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/godbus/dbus/v5"
	"github.com/tuklu/nZlauncher/internal/platform"
)

// Keep awake is two holds, each on its own timer:
//
//   - screen: an org.freedesktop.ScreenSaver inhibit, the one hypridle, KDE
//     and GNOME all honour. No idle lock, no blanking.
//   - sleep: a logind block inhibitor on sleep and the lid switch. The screen
//     may still lock and blank, but the machine never suspends, lid closed
//     or not, so a long job keeps running.
//
// Some services drop an inhibit when the caller's connection closes, and the
// logind one lives exactly as long as its file descriptor, so only the daemon
// holds them.

type Hold string

const (
	HoldScreen Hold = "screen"
	HoldSleep  Hold = "sleep"
)

// Holds lists every hold, in the order they are shown.
var Holds = []Hold{HoldScreen, HoldSleep}

var (
	ErrNoIdle   = errors.New("nothing serves org.freedesktop.ScreenSaver (hypridle, KDE, GNOME)")
	ErrNoLogind = errors.New("logind is not running, nothing can block sleep")
)

type heldState struct {
	release func() error
	until   time.Time // zero: until turned off
	timer   *time.Timer
}

var awake struct {
	sync.Mutex
	holds map[Hold]*heldState
}

func screenSaver() (dbus.BusObject, error) {
	if platform.Current().Idle == "" {
		return nil, ErrNoIdle
	}
	conn, err := dbus.SessionBus()
	if err != nil {
		return nil, err
	}
	return conn.Object("org.freedesktop.ScreenSaver", "/org/freedesktop/ScreenSaver"), nil
}

func acquire(h Hold) (func() error, error) {
	switch h {
	case HoldScreen:
		ss, err := screenSaver()
		if err != nil {
			return nil, err
		}
		var cookie uint32
		if err := ss.Call("org.freedesktop.ScreenSaver.Inhibit", 0, "nzlauncher", "keep awake").Store(&cookie); err != nil {
			return nil, err
		}
		return func() error {
			ss, err := screenSaver()
			if err != nil {
				return err
			}
			return ss.Call("org.freedesktop.ScreenSaver.UnInhibit", 0, cookie).Err
		}, nil
	case HoldSleep:
		if !logindRunning() {
			return nil, ErrNoLogind
		}
		conn, err := dbus.SystemBus()
		if err != nil {
			return nil, err
		}
		var fd dbus.UnixFD
		login := conn.Object("org.freedesktop.login1", "/org/freedesktop/login1")
		if err := login.Call("org.freedesktop.login1.Manager.Inhibit", 0,
			"sleep:handle-lid-switch", "nzlauncher", "keep running", "block").Store(&fd); err != nil {
			return nil, err
		}
		inhibitor := os.NewFile(uintptr(fd), "logind-inhibitor")
		return inhibitor.Close, nil
	}
	return nil, fmt.Errorf("unknown hold %q", h)
}

// logindRunning is sd_booted plus logind: its runtime directory exists.
func logindRunning() bool {
	_, err := os.Stat("/run/systemd/seats")
	return err == nil
}

// KeepAwake takes hold h for d, or until Release with d 0. Calling it again
// replaces the duration and keeps the hold.
func KeepAwake(h Hold, d time.Duration) error {
	awake.Lock()
	defer awake.Unlock()
	if awake.holds == nil {
		awake.holds = map[Hold]*heldState{}
	}
	state := awake.holds[h]
	if state == nil {
		release, err := acquire(h)
		if err != nil {
			return err
		}
		state = &heldState{release: release}
		awake.holds[h] = state
	}
	if state.timer != nil {
		state.timer.Stop()
		state.timer = nil
	}
	state.until = time.Time{}
	if d > 0 {
		state.until = time.Now().Add(d)
		var timer *time.Timer
		timer = time.AfterFunc(d, func() {
			awake.Lock()
			defer awake.Unlock()
			// a newer KeepAwake replaced this timer: its hold is not ours to end
			if cur := awake.holds[h]; cur != nil && cur.timer == timer {
				releaseLocked(h)
			}
		})
		state.timer = timer
	}
	return nil
}

// Release lets go of hold h. Not holding it is not an error.
func Release(h Hold) error {
	awake.Lock()
	defer awake.Unlock()
	return releaseLocked(h)
}

func releaseLocked(h Hold) error {
	state := awake.holds[h]
	if state == nil {
		return nil
	}
	if state.timer != nil {
		state.timer.Stop()
	}
	delete(awake.holds, h)
	return state.release()
}

// ReleaseAll lets go of every hold.
func ReleaseAll() error {
	awake.Lock()
	defer awake.Unlock()
	var errs []error
	for _, h := range Holds {
		errs = append(errs, releaseLocked(h))
	}
	return errors.Join(errs...)
}

// Held reports whether this process holds h, and until when.
func Held(h Hold) (held bool, until time.Time) {
	awake.Lock()
	defer awake.Unlock()
	state := awake.holds[h]
	if state == nil {
		return false, time.Time{}
	}
	return true, state.until
}

// Available reports whether hold h can work here.
func Available(h Hold) bool {
	switch h {
	case HoldScreen:
		return platform.Current().Idle != ""
	case HoldSleep:
		return logindRunning()
	}
	return false
}

// ApplyAwake runs one keep-awake request as the daemon receives it:
//
//	off               release every hold
//	<secs>            screen for secs, 0 for until turned off
//	<hold>:<secs>     screen, sleep or all for secs
//	<hold>:off        release that hold, all for every hold
func ApplyAwake(spec string) error {
	which, value, scoped := strings.Cut(spec, ":")
	if !scoped {
		which, value = string(HoldScreen), spec
		if spec == "off" {
			which = "all"
		}
	}
	var holds []Hold
	switch which {
	case "all":
		holds = Holds
	case string(HoldScreen), string(HoldSleep):
		holds = []Hold{Hold(which)}
	default:
		return fmt.Errorf("keep awake holds screen, sleep or all, not %q", which)
	}
	if value == "off" {
		var errs []error
		for _, h := range holds {
			errs = append(errs, Release(h))
		}
		return errors.Join(errs...)
	}
	secs, err := strconv.Atoi(value)
	if err != nil || secs < 0 {
		return errors.New("keep awake takes off or a number of seconds")
	}
	for _, h := range holds {
		if err := KeepAwake(h, time.Duration(secs)*time.Second); err != nil {
			return err
		}
	}
	return nil
}

func detached() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Setsid: true}
}
