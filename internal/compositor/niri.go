package compositor

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
	"time"
)

// niri takes one JSON request per line on $NIRI_SOCKET and answers one JSON
// line: {"Ok": ...} or {"Err": "..."}. After "EventStream" the connection
// carries one event per line.

func niriDial() (net.Conn, error) {
	path := os.Getenv("NIRI_SOCKET")
	if path == "" {
		return nil, errors.New("not running under niri")
	}
	return net.DialTimeout("unix", path, time.Second)
}

// niriAsk sends one request and returns what is under "Ok".
func niriAsk(request any) (json.RawMessage, error) {
	conn, err := niriDial()
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(time.Second))
	line, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}
	if _, err := conn.Write(append(line, '\n')); err != nil {
		return nil, err
	}
	reply, err := bufio.NewReader(conn).ReadBytes('\n')
	if err != nil {
		return nil, err
	}
	return niriOk(reply)
}

func niriOk(reply []byte) (json.RawMessage, error) {
	var r struct {
		Ok  json.RawMessage `json:"Ok"`
		Err *string         `json:"Err"`
	}
	if err := json.Unmarshal(reply, &r); err != nil {
		return nil, err
	}
	if r.Err != nil {
		return nil, errors.New("niri: " + *r.Err)
	}
	return r.Ok, nil
}

type niriWindow struct {
	ID    uint64  `json:"id"`
	Title *string `json:"title"`
	AppID *string `json:"app_id"`
}

func (w niriWindow) window() Window {
	out := Window{ID: strconv.FormatUint(w.ID, 10)}
	if w.Title != nil {
		out.Title = *w.Title
	}
	if w.AppID != nil {
		out.Class = *w.AppID
	}
	return out
}

// niriAction runs an action; niri answers "Handled".
func niriAction(action map[string]any) error {
	_, err := niriAsk(map[string]any{"Action": action})
	return err
}

func niriWorkspace(n int) map[string]any {
	return map[string]any{"Index": n}
}

var niri = backend{
	windows: func() ([]Window, error) {
		ok, err := niriAsk("Windows")
		if err != nil {
			return nil, err
		}
		var reply struct {
			Windows []niriWindow `json:"Windows"`
		}
		if err := json.Unmarshal(ok, &reply); err != nil {
			return nil, err
		}
		ws := make([]Window, 0, len(reply.Windows))
		for _, w := range reply.Windows {
			ws = append(ws, w.window())
		}
		return ws, nil
	},
	focus: func(id string) error {
		n, err := strconv.ParseUint(id, 10, 64)
		if err != nil {
			return fmt.Errorf("niri: %q is no window id", id)
		}
		return niriAction(map[string]any{"FocusWindow": map[string]any{"id": n}})
	},
	workspace: func(n int) error {
		return niriAction(map[string]any{"FocusWorkspace": map[string]any{"reference": niriWorkspace(n)}})
	},
	moveTo: func(n int) error {
		return niriAction(map[string]any{"MoveWindowToWorkspace": map[string]any{
			"window_id": nil, "reference": niriWorkspace(n), "focus": true,
		}})
	},
	focused: func() (Window, error) {
		ok, err := niriAsk("FocusedWindow")
		if err != nil {
			return Window{}, err
		}
		var reply struct {
			FocusedWindow *niriWindow `json:"FocusedWindow"`
		}
		if err := json.Unmarshal(ok, &reply); err != nil || reply.FocusedWindow == nil {
			return Window{}, err
		}
		return reply.FocusedWindow.window(), nil
	},
	watch: func(changed func()) error {
		conn, err := niriDial()
		if err != nil {
			return err
		}
		defer conn.Close()
		if _, err := conn.Write([]byte("\"EventStream\"\n")); err != nil {
			return err
		}
		lines := bufio.NewScanner(conn)
		lines.Buffer(make([]byte, 64*1024), 4*1024*1024)
		if !lines.Scan() {
			return lines.Err()
		}
		if _, err := niriOk(lines.Bytes()); err != nil {
			return err
		}
		changed()
		for lines.Scan() {
			// each event is {"Name": {...}}; the window ones say Window
			if bytes.Contains(lines.Bytes()[:min(len(lines.Bytes()), 40)], []byte("Window")) {
				changed()
			}
		}
		return lines.Err()
	},
	onScreen: func(markers []string) bool {
		for _, request := range []string{"Windows", "Layers"} {
			ok, err := niriAsk(request)
			if err != nil {
				continue
			}
			for _, m := range markers {
				if bytes.Contains(ok, []byte(m)) {
					return true
				}
			}
		}
		return false
	},
}
