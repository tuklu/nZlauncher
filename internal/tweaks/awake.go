package tweaks

import (
	"errors"
	"sync"
	"syscall"
	"time"

	"github.com/godbus/dbus/v5"
	"github.com/tuklu/nZlauncher/internal/platform"
)

// Keep awake holds an org.freedesktop.ScreenSaver inhibit, the one hypridle,
// KDE and GNOME all honour. Some services drop an inhibit when the caller's
// connection closes, so only the daemon holds it, optionally on a timer.

var ErrNoIdle = errors.New("nothing serves org.freedesktop.ScreenSaver (hypridle, KDE, GNOME)")

var awake struct {
	sync.Mutex
	held   bool
	cookie uint32
	until  time.Time // zero: until turned off
	timer  *time.Timer
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

// KeepAwake stops idle locking and screen blanking for d, or until
// AllowSleep with d 0. Calling it again replaces the duration.
func KeepAwake(d time.Duration) error {
	awake.Lock()
	defer awake.Unlock()
	if !awake.held {
		ss, err := screenSaver()
		if err != nil {
			return err
		}
		var cookie uint32
		if err := ss.Call("org.freedesktop.ScreenSaver.Inhibit", 0, "nzlauncher", "keep awake").Store(&cookie); err != nil {
			return err
		}
		awake.held, awake.cookie = true, cookie
	}
	if awake.timer != nil {
		awake.timer.Stop()
		awake.timer = nil
	}
	awake.until = time.Time{}
	if d > 0 {
		awake.until = time.Now().Add(d)
		awake.timer = time.AfterFunc(d, func() { AllowSleep() })
	}
	return nil
}

func AllowSleep() error {
	awake.Lock()
	defer awake.Unlock()
	if awake.timer != nil {
		awake.timer.Stop()
		awake.timer = nil
	}
	if !awake.held {
		return nil
	}
	awake.held = false
	ss, err := screenSaver()
	if err != nil {
		return err
	}
	return ss.Call("org.freedesktop.ScreenSaver.UnInhibit", 0, awake.cookie).Err
}

// Awake reports whether this process holds the screen awake, and until when.
func Awake() (held bool, until time.Time) {
	awake.Lock()
	defer awake.Unlock()
	return awake.held, awake.until
}

// IdleAvailable reports whether keep awake can work here.
func IdleAvailable() bool {
	return platform.Current().Idle != ""
}

func detached() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Setsid: true}
}
