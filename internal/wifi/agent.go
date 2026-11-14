package wifi

import (
	"errors"
	"sync/atomic"

	"github.com/godbus/dbus/v5"
)

// Joining a new secured network: nzlauncher registers itself as the iwd
// agent for the one connection, so iwd asks it for the passphrase and the
// answer goes straight back over D-Bus, never through argv or a terminal.
// NetworkManager has its own secret agents and keeps the terminal path.

// ErrNoAgent means this backend cannot ask for a passphrase here; use its
// terminal tool instead.
var ErrNoAgent = errors.New("this Wi-Fi service takes the passphrase in its own tool")

// ErrCanceled is a prompt closed without an answer.
var ErrCanceled = errors.New("canceled")

// Ask shows prompt and returns what was typed, ErrCanceled when dismissed.
type Ask func(prompt string) (string, error)

const agentPath = dbus.ObjectPath("/dev/tuklu/nzlauncher/agent")

// Join connects to ssid, asking for the passphrase with ask if iwd wants one.
func Join(ssid string, ask Ask) error {
	_, name, ok := current()
	if !ok {
		return ErrNoBackend
	}
	if name != "iwd" {
		return ErrNoAgent
	}
	conn, err := dbus.ConnectSystemBus()
	if err != nil {
		return err
	}
	defer conn.Close()

	a := &iwdAgent{ssid: ssid, ask: ask}
	if err := conn.Export(a, agentPath, "net.connman.iwd.Agent"); err != nil {
		return err
	}
	manager := conn.Object(iwdBus, "/net/connman/iwd")
	if err := manager.Call("net.connman.iwd.AgentManager.RegisterAgent", 0, agentPath).Err; err != nil {
		return err
	}
	defer manager.Call("net.connman.iwd.AgentManager.UnregisterAgent", 0, agentPath)

	if err := iwdConnect(conn, ssid); err != nil {
		if a.canceled.Load() {
			return ErrCanceled
		}
		return err
	}
	return nil
}

// iwdAgent answers iwd's net.connman.iwd.Agent calls. Only the pre-shared
// key is asked for; enterprise networks need identities a dialog cannot
// sensibly collect, so they are declined and left to iwctl.
type iwdAgent struct {
	ssid     string
	ask      Ask
	canceled atomic.Bool // set on the bus goroutine, read after Connect returns
}

var errAgentCanceled = dbus.NewError("net.connman.iwd.Agent.Error.Canceled", []any{"canceled"})

func (a *iwdAgent) RequestPassphrase(network dbus.ObjectPath) (string, *dbus.Error) {
	pass, err := a.ask("Passphrase for " + a.ssid)
	if err != nil {
		a.canceled.Store(true)
		return "", errAgentCanceled
	}
	return pass, nil
}

func (a *iwdAgent) RequestPrivateKeyPassphrase(dbus.ObjectPath) (string, *dbus.Error) {
	return "", errAgentCanceled
}

func (a *iwdAgent) RequestUserNameAndPassword(dbus.ObjectPath) (string, string, *dbus.Error) {
	return "", "", errAgentCanceled
}

func (a *iwdAgent) RequestUserPassword(dbus.ObjectPath, string) (string, *dbus.Error) {
	return "", errAgentCanceled
}

func (a *iwdAgent) Release() *dbus.Error { return nil }

func (a *iwdAgent) Cancel(string) *dbus.Error {
	a.canceled.Store(true)
	return nil
}
