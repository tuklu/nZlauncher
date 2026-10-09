package main

import (
	"bytes"
	"os"
	"os/exec"
	"strconv"
	"syscall"
	"time"
)

func launch(command string) error {
	return startDetached(exec.Command("sh", "-c", command))
}

func startDetached(cmd *exec.Cmd) error {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	devnull, err := os.OpenFile(os.DevNull, os.O_RDWR, 0)
	if err == nil {
		cmd.Stdin, cmd.Stdout, cmd.Stderr = devnull, devnull, devnull
	}
	return cmd.Start()
}

// Window class (TUI, GTK window mode) and layer namespace (GTK overlay)
// that mark the launcher as still on screen.
var launcherSurfaces = [][]byte{
	[]byte(`"nz.launcher"`),
	[]byte(`"dev.tuklu.nz"`),
	[]byte(`"nz-bar"`),
}

const (
	closeWaitLimit = 2 * time.Second
	closePoll      = 25 * time.Millisecond
	// Hyprland keeps rendering a closed window during its fade-out
	closeAnimationSettle = 120 * time.Millisecond
)

// The launcher must be gone before the action runs, or screenshots, OCR,
// and pickers capture it and keyboard-driven actions hit it instead of
// the window underneath. Hands off to a detached `nz after-close`.
func launchAfterClose(command string) error {
	self, err := os.Executable()
	if err != nil {
		return launch(command)
	}
	return startDetached(exec.Command(self, "after-close", strconv.Itoa(os.Getpid()), command))
}

func runAfterClose(args []string) error {
	if len(args) != 2 {
		usage()
	}
	pid, err := strconv.Atoi(args[0])
	if err != nil {
		return err
	}

	deadline := time.Now().Add(closeWaitLimit)
	for time.Now().Before(deadline) && processAlive(pid) {
		time.Sleep(closePoll)
	}
	for time.Now().Before(deadline) && launcherOnScreen() {
		time.Sleep(closePoll)
	}
	time.Sleep(closeAnimationSettle)

	return syscall.Exec("/bin/sh", []string{"sh", "-c", args[1]}, os.Environ())
}

func processAlive(pid int) bool {
	return syscall.Kill(pid, 0) == nil
}

func launcherOnScreen() bool {
	for _, what := range []string{"clients", "layers"} {
		out, err := exec.Command("hyprctl", what, "-j").Output()
		if err != nil {
			return false
		}
		for _, marker := range launcherSurfaces {
			if bytes.Contains(out, marker) {
				return true
			}
		}
	}
	return false
}
