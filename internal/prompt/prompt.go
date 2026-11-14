// Package prompt asks the user for a secret: on the terminal when there is
// one, otherwise in a desktop dialog.
package prompt

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"golang.org/x/sys/unix"
)

// ErrCanceled is a prompt closed or answered with nothing.
var ErrCanceled = errors.New("canceled")

// ErrNoWay means there is no terminal and no dialog to ask with.
var ErrNoWay = errors.New("no terminal and no password dialog (zenity or kdialog) to ask with")

// Secret asks for a secret, the terminal first. The answer is never echoed.
func Secret(title string) (string, error) {
	if isTerminal(os.Stdin.Fd()) {
		return fromTerminal(title)
	}
	if ask, ok := Dialog(); ok {
		return ask(title)
	}
	return "", ErrNoWay
}

// Dialog finds a password dialog for the graphical session.
func Dialog() (func(title string) (string, error), bool) {
	if os.Getenv("WAYLAND_DISPLAY") == "" && os.Getenv("DISPLAY") == "" {
		return nil, false
	}
	dialogs := []struct {
		name string
		args func(title string) []string
	}{
		{"zenity", func(t string) []string { return []string{"--password", "--title", t} }},
		{"kdialog", func(t string) []string { return []string{"--password", t, "--title", "nzlauncher"} }},
	}
	for _, d := range dialogs {
		path, err := exec.LookPath(d.name)
		if err != nil {
			continue
		}
		return func(title string) (string, error) {
			out, err := exec.Command(path, d.args(title)...).Output()
			answer := strings.TrimSuffix(string(out), "\n")
			if err != nil || answer == "" {
				return "", ErrCanceled
			}
			return answer, nil
		}, true
	}
	return nil, false
}

func isTerminal(fd uintptr) bool {
	_, err := unix.IoctlGetTermios(int(fd), unix.TCGETS)
	return err == nil
}

// fromTerminal reads a line with echo off, restoring the terminal after.
func fromTerminal(title string) (string, error) {
	fd := int(os.Stdin.Fd())
	saved, err := unix.IoctlGetTermios(fd, unix.TCGETS)
	if err != nil {
		return "", err
	}
	quiet := *saved
	quiet.Lflag &^= unix.ECHO
	if err := unix.IoctlSetTermios(fd, unix.TCSETS, &quiet); err != nil {
		return "", err
	}
	defer unix.IoctlSetTermios(fd, unix.TCSETS, saved)

	fmt.Fprintf(os.Stderr, "%s: ", title)
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	fmt.Fprintln(os.Stderr)
	answer := strings.TrimSuffix(line, "\n")
	if err != nil || answer == "" {
		return "", ErrCanceled
	}
	return answer, nil
}
