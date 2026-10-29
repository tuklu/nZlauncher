// Package server keeps the engine resident and answers queries over a unix
// socket, so frontends stay thin. The protocol is documented in
// docs/protocol.md: one JSON object per line each way, connections reusable.
package server

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"runtime/debug"
	"strconv"
	"syscall"
	"time"

	"github.com/tuklu/nZlauncher/internal/action"
	"github.com/tuklu/nZlauncher/internal/answer"
	"github.com/tuklu/nZlauncher/internal/config"
	"github.com/tuklu/nZlauncher/internal/engine"
	"github.com/tuklu/nZlauncher/internal/files"
	"github.com/tuklu/nZlauncher/internal/providers"
	"github.com/tuklu/nZlauncher/internal/tweaks"
)

type Request struct {
	Op      string `json:"op"`                // "query", "ping" or "awake"
	Q       string `json:"q,omitempty"`       // the query text
	Focused string `json:"focused,omitempty"` // class of the window the user came from
}

type Result struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Icon     string `json:"icon,omitempty"`
	IconName string `json:"icon_name,omitempty"`
	Group    string `json:"group,omitempty"`
	Cmd      string `json:"cmd"`
	Display  string `json:"display"`
	Hint     string `json:"hint,omitempty"`
	Preview  string `json:"preview,omitempty"`
}

type Response struct {
	OK      bool     `json:"ok"`
	Error   string   `json:"error,omitempty"`
	Version string   `json:"version,omitempty"`
	Results []Result `json:"results,omitempty"`
}

func SocketPath() string {
	dir := os.Getenv("XDG_RUNTIME_DIR")
	if dir == "" {
		dir = fmt.Sprintf("/run/user/%d", os.Getuid())
	}
	return filepath.Join(dir, "nzlauncher.sock")
}

func version() string {
	if info, ok := debug.ReadBuildInfo(); ok {
		return info.Main.Version
	}
	return "unknown"
}

// Serve listens until SIGINT or SIGTERM. A socket left by a dead daemon is
// replaced; a live one means another daemon already serves this session.
// keepRatesFresh refreshes the currency cache; the call itself does nothing
// while the cache is under a day old, so checking often is cheap and a
// laptop that was offline catches up soon after it is back.
func keepRatesFresh() {
	for {
		if providers.On("currency") {
			if err := answer.RefreshRates(); err != nil {
				log.Printf("currency rates: %v", err)
			}
		}
		time.Sleep(30 * time.Minute)
	}
}

func Serve() error {
	path := SocketPath()
	if conn, err := net.DialTimeout("unix", path, 100*time.Millisecond); err == nil {
		conn.Close()
		return errors.New("already running on " + path)
	}
	os.Remove(path)

	ln, err := net.Listen("unix", path)
	if err != nil {
		return err
	}
	if err := os.Chmod(path, 0o600); err != nil {
		ln.Close()
		return err
	}
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-stop
		ln.Close()
	}()
	defer os.Remove(path)

	startFileIndex()
	engine.Warm()
	go func() {
		providers.PruneClipCache()
		providers.WarmClipboard()
	}()
	go keepRatesFresh()
	for {
		conn, err := ln.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return nil
			}
			return err
		}
		go handle(conn)
	}
}

func handle(conn net.Conn) {
	defer conn.Close()
	in := bufio.NewScanner(conn)
	in.Buffer(make([]byte, 64*1024), 1024*1024)
	out := json.NewEncoder(conn)
	for in.Scan() {
		var req Request
		if err := json.Unmarshal(in.Bytes(), &req); err != nil {
			out.Encode(Response{Error: "bad request: " + err.Error()})
			continue
		}
		out.Encode(respond(req))
	}
}

func respond(req Request) Response {
	switch req.Op {
	case "ping":
		return Response{OK: true, Version: version()}
	case "query":
		acts, err := engine.QueryFor(req.Q, req.Focused)
		if err != nil {
			return Response{Error: err.Error()}
		}
		return Response{OK: true, Results: results(acts)}
	case "awake":
		// "off", or how long in seconds, 0 for until turned off
		var err error
		if req.Q == "off" {
			err = tweaks.AllowSleep()
		} else if secs, convErr := strconv.Atoi(req.Q); convErr == nil && secs >= 0 {
			err = tweaks.KeepAwake(time.Duration(secs) * time.Second)
		} else {
			err = errors.New("awake takes off or a number of seconds")
		}
		if err != nil {
			return Response{Error: err.Error()}
		}
		return Response{OK: true}
	}
	return Response{Error: "unknown op: " + req.Op}
}

func results(acts []action.Action) []Result {
	out := make([]Result, len(acts))
	for i, a := range acts {
		out[i] = Result{ID: a.ID, Name: a.Name, Icon: a.Icon, IconName: a.IconName, Group: a.Group, Cmd: a.Cmd, Display: a.Display(), Hint: a.Hint, Preview: a.Preview}
	}
	return out
}

// Ask sends one request to a running daemon. It fails fast when there is
// none, so callers can fall back to running the engine themselves.
func Ask(req Request) (Response, error) {
	conn, err := net.DialTimeout("unix", SocketPath(), 50*time.Millisecond)
	if err != nil {
		return Response{}, err
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(5 * time.Second))
	if err := json.NewEncoder(conn).Encode(req); err != nil {
		return Response{}, err
	}
	var resp Response
	if err := json.NewDecoder(bufio.NewReader(conn)).Decode(&resp); err != nil {
		return Response{}, err
	}
	if !resp.OK {
		return resp, errors.New(resp.Error)
	}
	return resp, nil
}

// FaceSocketPath is where a resident GTK face listens for the bar.
func FaceSocketPath() string {
	return filepath.Join(filepath.Dir(SocketPath()), "nzlauncher-gtk.sock")
}

// ToggleFace asks a resident GTK face to show (with this query and focus
// context) or hide. It fails fast when no face is running.
func ToggleFace(q, focused string) error {
	conn, err := net.DialTimeout("unix", FaceSocketPath(), 50*time.Millisecond)
	if err != nil {
		return err
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(2 * time.Second))
	if err := json.NewEncoder(conn).Encode(Request{Op: "toggle", Q: q, Focused: focused}); err != nil {
		return err
	}
	var resp Response
	if err := json.NewDecoder(conn).Decode(&resp); err != nil {
		return err
	}
	if !resp.OK {
		return errors.New(resp.Error)
	}
	return nil
}

// ListenFace binds the face socket, replacing a stale one. It refuses when a
// live face already owns it.
func ListenFace() (net.Listener, error) {
	path := FaceSocketPath()
	if conn, err := net.DialTimeout("unix", path, 100*time.Millisecond); err == nil {
		conn.Close()
		return nil, errors.New("a resident face already runs on " + path)
	}
	os.Remove(path)
	ln, err := net.Listen("unix", path)
	if err != nil {
		return nil, err
	}
	if err := os.Chmod(path, 0o600); err != nil {
		ln.Close()
		return nil, err
	}
	return ln, nil
}

// The file index lives as long as the daemon, with config.toml's roots and
// excludes when set.
func startFileIndex() {
	c := files.DefaultConfig()
	if s, err := config.LoadSettings(); err == nil {
		if roots := s.FileRoots(); len(roots) > 0 {
			c.Roots = roots
		}
		if len(s.Files.Exclude) > 0 {
			c.Exclude = s.Files.Exclude
		}
	}
	files.Start(c)
}

// Query is what a face calls per keystroke: the daemon's answer when it
// runs, so every frontend shares its index and caches, otherwise the engine
// in this process.
func Query(q, focused string) ([]action.Action, error) {
	resp, err := Ask(Request{Op: "query", Q: q, Focused: focused})
	if err != nil && resp.Error == "" {
		return engine.QueryFor(q, focused)
	}
	if err != nil {
		return nil, err
	}
	acts := make([]action.Action, len(resp.Results))
	for i, r := range resp.Results {
		acts[i] = action.Action{ID: r.ID, Name: r.Name, Icon: r.Icon, IconName: r.IconName,
			Group: r.Group, Cmd: r.Cmd, Hint: r.Hint, Preview: r.Preview}
	}
	return acts, nil
}
