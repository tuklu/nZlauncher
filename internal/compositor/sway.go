package compositor

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"time"
)

// sway speaks the i3 IPC protocol on $SWAYSOCK: "i3-ipc", the payload
// length and the message type (both 32-bit, native order), then the payload.
const (
	i3RunCommand = 0
	i3Subscribe  = 2
	i3GetTree    = 4
	i3Magic      = "i3-ipc"
	i3EventBit   = 1 << 31
)

func i3Dial() (net.Conn, error) {
	path := os.Getenv("SWAYSOCK")
	if path == "" {
		return nil, errors.New("not running under sway")
	}
	return net.DialTimeout("unix", path, time.Second)
}

func i3Send(conn net.Conn, kind uint32, payload []byte) error {
	header := make([]byte, len(i3Magic)+8)
	copy(header, i3Magic)
	binary.NativeEndian.PutUint32(header[len(i3Magic):], uint32(len(payload)))
	binary.NativeEndian.PutUint32(header[len(i3Magic)+4:], kind)
	_, err := conn.Write(append(header, payload...))
	return err
}

func i3Read(conn net.Conn) (kind uint32, payload []byte, err error) {
	header := make([]byte, len(i3Magic)+8)
	if _, err := io.ReadFull(conn, header); err != nil {
		return 0, nil, err
	}
	if string(header[:len(i3Magic)]) != i3Magic {
		return 0, nil, errors.New("sway: not an i3-ipc reply")
	}
	size := binary.NativeEndian.Uint32(header[len(i3Magic):])
	kind = binary.NativeEndian.Uint32(header[len(i3Magic)+4:])
	payload = make([]byte, size)
	_, err = io.ReadFull(conn, payload)
	return kind, payload, err
}

// i3Ask sends one message and returns the reply.
func i3Ask(kind uint32, payload string) ([]byte, error) {
	conn, err := i3Dial()
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(time.Second))
	if err := i3Send(conn, kind, []byte(payload)); err != nil {
		return nil, err
	}
	_, reply, err := i3Read(conn)
	return reply, err
}

// i3Command runs sway commands and reports the first failure.
func i3Command(cmd string) error {
	reply, err := i3Ask(i3RunCommand, cmd)
	if err != nil {
		return err
	}
	var results []struct {
		Success bool   `json:"success"`
		Error   string `json:"error"`
	}
	if err := json.Unmarshal(reply, &results); err != nil {
		return err
	}
	for _, r := range results {
		if !r.Success {
			return errors.New("sway: " + r.Error)
		}
	}
	return nil
}

// swayNode is a node of sway's tree; windows are the leaves.
type swayNode struct {
	ID       int64      `json:"id"`
	Type     string     `json:"type"`
	Name     *string    `json:"name"`
	AppID    *string    `json:"app_id"`
	Focused  bool       `json:"focused"`
	Nodes    []swayNode `json:"nodes"`
	Floating []swayNode `json:"floating_nodes"`
	Props    *struct {
		Class string `json:"class"`
	} `json:"window_properties"`
}

func (n swayNode) window() (Window, bool) {
	isWindow := (n.Type == "con" || n.Type == "floating_con") && len(n.Nodes) == 0 && len(n.Floating) == 0 &&
		(n.AppID != nil || n.Props != nil)
	if !isWindow {
		return Window{}, false
	}
	w := Window{ID: strconv.FormatInt(n.ID, 10)}
	if n.Name != nil {
		w.Title = *n.Name
	}
	switch {
	case n.AppID != nil && *n.AppID != "":
		w.Class = *n.AppID
	case n.Props != nil:
		w.Class = n.Props.Class // an X11 window
	}
	return w, true
}

// swayWindows walks the tree; focused is the window with focus, if any.
func swayWindows(tree []byte) (all []Window, focused Window, err error) {
	var root swayNode
	if err := json.Unmarshal(tree, &root); err != nil {
		return nil, Window{}, err
	}
	var walk func(swayNode)
	walk = func(n swayNode) {
		if w, ok := n.window(); ok {
			all = append(all, w)
			if n.Focused {
				focused = w
			}
		}
		for _, c := range n.Nodes {
			walk(c)
		}
		for _, c := range n.Floating {
			walk(c)
		}
	}
	walk(root)
	return all, focused, nil
}

var sway = backend{
	windows: func() ([]Window, error) {
		tree, err := i3Ask(i3GetTree, "")
		if err != nil {
			return nil, err
		}
		all, _, err := swayWindows(tree)
		return all, err
	},
	focus: func(id string) error {
		if _, err := strconv.ParseInt(id, 10, 64); err != nil {
			return fmt.Errorf("sway: %q is no container id", id)
		}
		return i3Command("[con_id=" + id + "] focus")
	},
	workspace: func(n int) error {
		return i3Command(fmt.Sprintf("workspace number %d", n))
	},
	moveTo: func(n int) error {
		return i3Command(fmt.Sprintf("move container to workspace number %d", n))
	},
	focused: func() (Window, error) {
		tree, err := i3Ask(i3GetTree, "")
		if err != nil {
			return Window{}, err
		}
		_, focused, err := swayWindows(tree)
		return focused, err
	},
	watch: func(changed func()) error {
		conn, err := i3Dial()
		if err != nil {
			return err
		}
		defer conn.Close()
		if err := i3Send(conn, i3Subscribe, []byte(`["window"]`)); err != nil {
			return err
		}
		if _, _, err := i3Read(conn); err != nil { // the subscribe reply
			return err
		}
		changed()
		for {
			kind, _, err := i3Read(conn)
			if err != nil {
				return err
			}
			if kind&i3EventBit != 0 {
				changed()
			}
		}
	},
	onScreen: func(markers []string) bool {
		tree, err := i3Ask(i3GetTree, "")
		if err != nil {
			return false
		}
		for _, m := range markers {
			if bytes.Contains(tree, []byte(m)) {
				return true
			}
		}
		return false
	},
}
