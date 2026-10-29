// Package shell runs launcher actions as detached shell commands.
package shell

import (
	"fmt"
	"github.com/tuklu/nZlauncher/internal/compositor"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/tuklu/nZlauncher/internal/identity"
)

func Launch(command string) error {
	return StartDetached(exec.Command("sh", "-c", command))
}

func StartDetached(cmd *exec.Cmd) error {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	devnull, err := os.OpenFile(os.DevNull, os.O_RDWR, 0)
	if err == nil {
		cmd.Stdin, cmd.Stdout, cmd.Stderr = devnull, devnull, devnull
	}
	return cmd.Start()
}

// Window class (TUI, GTK window mode) and layer namespace (GTK overlay)
// that mark the launcher as still on screen.
var launcherSurfaces = []string{
	`"` + identity.TUIClass + `"`,
	`"` + identity.AppID + `"`,
	`"` + identity.Namespace + `"`,
}

const (
	closeWaitLimit = 2 * time.Second
	closePoll      = 25 * time.Millisecond
	// Hyprland keeps rendering a closed window during its fade-out
	closeAnimationSettle = 120 * time.Millisecond
)

// The launcher must be gone before the action runs, or screenshots, OCR,
// and pickers capture it and keyboard-driven actions hit it instead of
// the window underneath. Hands off to a detached `nzlauncher after-close`,
// which waits for this process to exit.
func LaunchAfterClose(command string) error {
	return launchAfter(os.Getpid(), command)
}

// LaunchAfterHide is for a face that stays running: wait only for its
// surface to leave the screen.
func LaunchAfterHide(command string) error {
	return launchAfter(0, command)
}

func launchAfter(pid int, command string) error {
	cli, err := CLI()
	if err != nil {
		return Launch(command)
	}
	return StartDetached(exec.Command(cli, "after-close", strconv.Itoa(pid), command))
}

// CLI is the nzlauncher binary: this one, the one beside it, or the one on
// PATH. The GTK face is a different binary and lacks the CLI's commands.
func CLI() (string, error) {
	if self, err := os.Executable(); err == nil {
		if filepath.Base(self) == "nzlauncher" {
			return self, nil
		}
		sibling := filepath.Join(filepath.Dir(self), "nzlauncher")
		if _, err := os.Stat(sibling); err == nil {
			return sibling, nil
		}
	}
	return exec.LookPath("nzlauncher")
}

// RunAfterClose is the `nz after-close <pid> <command>` entry point.
func RunAfterClose(args []string) error {
	if len(args) != 2 {
		return fmt.Errorf("after-close: want <pid> <command>, got %d args", len(args))
	}
	pid, err := strconv.Atoi(args[0])
	if err != nil {
		return err
	}

	deadline := time.Now().Add(closeWaitLimit)
	for pid > 0 && time.Now().Before(deadline) && processAlive(pid) {
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
	return compositor.OnScreen(launcherSurfaces)
}

func Quote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
