// Package hypr talks to Hyprland over its sockets directly. hyprctl does the
// same thing from a fresh process, which costs more than the request itself.
package hypr

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"
)

const timeout = time.Second

func socketDir() (string, error) {
	sig := os.Getenv("HYPRLAND_INSTANCE_SIGNATURE")
	if sig == "" {
		return "", errors.New("not running under Hyprland")
	}
	runtime := os.Getenv("XDG_RUNTIME_DIR")
	if runtime == "" {
		runtime = fmt.Sprintf("/run/user/%d", os.Getuid())
	}
	return filepath.Join(runtime, "hypr", sig), nil
}

// Request sends one command, as hyprctl would ("j/clients", "dispatch ...")
// and returns the reply.
func Request(cmd string) ([]byte, error) {
	dir, err := socketDir()
	if err != nil {
		return nil, err
	}
	conn, err := net.DialTimeout("unix", filepath.Join(dir, ".socket.sock"), timeout)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(timeout))
	if _, err := io.WriteString(conn, cmd); err != nil {
		return nil, err
	}
	return io.ReadAll(conn)
}

// JSON is Request for the read-only queries that answer in JSON.
func JSON(what string) ([]byte, error) {
	return Request("j/" + what)
}

// Lua-configured Hyprland reads `dispatch` arguments as Lua; hyprlang ones take
// the classic form. Try classic first and remember when Lua was needed.
var luaDispatch atomic.Bool

// Focus switches to a window by address, on either config language.
func Focus(address string) error {
	return Dispatch("focuswindow address:"+address, fmt.Sprintf(`hl.dsp.focus({ window = "address:%s" })`, address))
}

// Dispatch runs a dispatcher, in the classic form or, on a Lua config, the
// Lua one; which one worked is remembered.
func Dispatch(classic, lua string) error {
	if luaDispatch.Load() {
		return expectOK(Request("dispatch " + lua))
	}
	if err := expectOK(Request("dispatch " + classic)); err == nil {
		return nil
	}
	luaDispatch.Store(true)
	return expectOK(Request("dispatch " + lua))
}

func expectOK(reply []byte, err error) error {
	if err != nil {
		return err
	}
	if r := strings.TrimSpace(string(reply)); r != "ok" {
		return errors.New(r)
	}
	return nil
}

// Watch calls changed for every Hyprland event whose name passes keep, until
// the event socket closes. It is how long-lived processes learn that cached
// state went stale without polling.
func Watch(keep func(event string) bool, changed func()) error {
	dir, err := socketDir()
	if err != nil {
		return err
	}
	conn, err := net.Dial("unix", filepath.Join(dir, ".socket2.sock"))
	if err != nil {
		return err
	}
	defer conn.Close()
	// anything that happened before we were listening is unknown
	changed()
	lines := bufio.NewScanner(conn)
	for lines.Scan() {
		name, _, _ := strings.Cut(lines.Text(), ">>")
		if keep(name) {
			changed()
		}
	}
	return lines.Err()
}
