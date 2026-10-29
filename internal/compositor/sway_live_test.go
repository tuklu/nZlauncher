//go:build swaylive

package compositor

import (
	"slices"
	"testing"
	"time"
)

// Against a real headless sway, started by test/sway/run.sh.

func waitForWindow(t *testing.T, class string) Window {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		ws, err := Windows()
		if err != nil {
			t.Fatal(err)
		}
		if i := slices.IndexFunc(ws, func(w Window) bool { return w.Class == class }); i >= 0 {
			return ws[i]
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("no %s window appeared", class)
	return Window{}
}

func TestSwayLive(t *testing.T) {
	if !Supported() {
		t.Fatal("sway not detected")
	}
	changed := make(chan struct{}, 16)
	go Watch(func() { changed <- struct{}{} })
	<-changed // the call at the start

	if err := i3Command("exec foot --app-id nz-test-a sleep 60"); err != nil {
		t.Fatal(err)
	}
	a := waitForWindow(t, "nz-test-a")
	select {
	case <-changed:
	case <-time.After(5 * time.Second):
		t.Error("no window event for the new window")
	}

	if err := Workspace(2); err != nil {
		t.Fatal(err)
	}
	if err := Focus(a.ID); err != nil {
		t.Fatal(err)
	}
	if f, err := Focused(); err != nil || f.ID != a.ID {
		t.Errorf("focused %+v, %v", f, err)
	}
	if err := MoveToWorkspace(3); err != nil {
		t.Fatal(err)
	}
	if !OnScreen([]string{`"nz-test-a"`}) {
		t.Error("the window is not on screen")
	}
}
