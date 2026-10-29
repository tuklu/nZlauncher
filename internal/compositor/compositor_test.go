package compositor

import (
	"bufio"
	"encoding/json"
	"net"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// fakeSocket listens on a unix socket and answers each connection with serve.
func fakeSocket(t *testing.T, serve func(net.Conn)) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "sock")
	ln, err := net.Listen("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func() {
				defer conn.Close()
				serve(conn)
			}()
		}
	}()
	return path
}

const swayTree = `{"id":1,"type":"root","nodes":[{"id":2,"type":"output","nodes":[{"id":3,"type":"workspace","name":"1","nodes":[
	{"id":10,"type":"con","name":"notes - foot","app_id":"foot","focused":true,"nodes":[],"floating_nodes":[]},
	{"id":11,"type":"con","name":"X app","app_id":null,"window_properties":{"class":"Xclock"},"nodes":[],"floating_nodes":[]}
],"floating_nodes":[{"id":12,"type":"floating_con","name":"float","app_id":"pavucontrol","nodes":[],"floating_nodes":[]}]}]}]}`

func TestSway(t *testing.T) {
	var commands []string
	path := fakeSocket(t, func(conn net.Conn) {
		for {
			kind, payload, err := i3Read(conn)
			if err != nil {
				return
			}
			switch kind {
			case i3GetTree:
				i3Send(conn, i3GetTree, []byte(swayTree))
			case i3RunCommand:
				commands = append(commands, string(payload))
				i3Send(conn, i3RunCommand, []byte(`[{"success":true}]`))
			case i3Subscribe:
				i3Send(conn, i3Subscribe, []byte(`{"success":true}`))
				i3Send(conn, i3EventBit|3, []byte(`{"change":"new"}`))
				return
			}
		}
	})
	t.Setenv("SWAYSOCK", path)

	ws, err := sway.windows()
	want := []Window{{ID: "10", Class: "foot", Title: "notes - foot", Workspace: "1"}, {ID: "11", Class: "Xclock", Title: "X app", Workspace: "1"}, {ID: "12", Class: "pavucontrol", Title: "float", Workspace: "1"}}
	if err != nil || !slices.Equal(ws, want) {
		t.Fatalf("windows %+v, %v", ws, err)
	}
	if f, _ := sway.focused(); f.ID != "10" {
		t.Errorf("focused %+v", f)
	}
	sway.focus("11")
	sway.workspace(3)
	sway.moveTo(4)
	if want := []string{"[con_id=11] focus", "workspace number 3", "move container to workspace number 4"}; !slices.Equal(commands, want) {
		t.Errorf("commands %q", commands)
	}
	if sway.focus("11; exec rm") == nil {
		t.Errorf("a non-numeric id reached sway")
	}
	calls := 0
	sway.watch(func() { calls++ })
	if calls != 2 { // once at the start, once for the event
		t.Errorf("changed called %d times", calls)
	}
	if !sway.onScreen([]string{`"pavucontrol"`}) || sway.onScreen([]string{`"nzlauncher"`}) {
		t.Errorf("onScreen")
	}
}

func TestNiri(t *testing.T) {
	var actions []string
	path := fakeSocket(t, func(conn net.Conn) {
		lines := bufio.NewScanner(conn)
		for lines.Scan() {
			req := lines.Text()
			switch {
			case req == `"Windows"`:
				conn.Write([]byte(`{"Ok":{"Windows":[{"id":7,"title":"mail","app_id":"thunderbird","workspace_id":1,"is_focused":false},{"id":8,"title":null,"app_id":"foot","is_focused":true}]}}` + "\n"))
			case req == `"FocusedWindow"`:
				conn.Write([]byte(`{"Ok":{"FocusedWindow":{"id":8,"title":null,"app_id":"foot"}}}` + "\n"))
			case req == `"Layers"`:
				conn.Write([]byte(`{"Ok":{"Layers":[{"namespace":"waybar"}]}}` + "\n"))
			case req == `"EventStream"`:
				conn.Write([]byte(`{"Ok":"Handled"}` + "\n" +
					`{"WorkspacesChanged":{"workspaces":[]}}` + "\n" +
					`{"WindowOpenedOrChanged":{"window":{"id":9}}}` + "\n" +
					`{"WindowClosed":{"id":9}}` + "\n"))
				return
			case strings.HasPrefix(req, `{"Action"`):
				actions = append(actions, req)
				conn.Write([]byte(`{"Ok":"Handled"}` + "\n"))
			default:
				conn.Write([]byte(`{"Err":"unknown request"}` + "\n"))
			}
		}
	})
	t.Setenv("NIRI_SOCKET", path)

	ws, err := niri.windows()
	if err != nil || !slices.Equal(ws, []Window{{ID: "7", Class: "thunderbird", Title: "mail", Workspace: "1"}, {ID: "8", Class: "foot", Title: ""}}) {
		t.Fatalf("windows %+v, %v", ws, err)
	}
	if f, _ := niri.focused(); f.ID != "8" || f.Class != "foot" {
		t.Errorf("focused %+v", f)
	}
	niri.focus("7")
	niri.workspace(2)
	niri.moveTo(3)
	want := []map[string]any{
		{"Action": map[string]any{"FocusWindow": map[string]any{"id": 7.0}}},
		{"Action": map[string]any{"FocusWorkspace": map[string]any{"reference": map[string]any{"Index": 2.0}}}},
		{"Action": map[string]any{"MoveWindowToWorkspace": map[string]any{"window_id": nil, "reference": map[string]any{"Index": 3.0}, "focus": true}}},
	}
	if len(actions) != len(want) {
		t.Fatalf("actions %q", actions)
	}
	for i, a := range actions {
		var got map[string]any
		json.Unmarshal([]byte(a), &got)
		g, _ := json.Marshal(got)
		w, _ := json.Marshal(want[i])
		if string(g) != string(w) {
			t.Errorf("action %d: %s, want %s", i, g, w)
		}
	}
	calls := 0
	niri.watch(func() { calls++ })
	if calls != 3 { // the start and the two window events, not the workspace one
		t.Errorf("changed called %d times", calls)
	}
	if !niri.onScreen([]string{`"waybar"`}) || niri.onScreen([]string{`"nzlauncher"`}) {
		t.Errorf("onScreen")
	}
}
