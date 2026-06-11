package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Community providers: drop an executable in ~/.config/neZha/providers.d/.
// Its filename is the trigger word; it is invoked with the rest of the query
// as $1 and prints one action per line:
//
//	id <TAB> shell-command <TAB> display text
func pluginActions(word, q string) ([]Action, bool) {
	path := filepath.Join(os.Getenv("HOME"), ".config/neZha/providers.d", word)
	info, err := os.Stat(path)
	if err != nil || info.IsDir() || info.Mode()&0o111 == 0 {
		return nil, false
	}

	out, err := exec.Command(path, q).Output()
	if err != nil {
		return nil, true
	}

	var acts []Action
	for _, line := range strings.Split(strings.TrimRight(string(out), "\n"), "\n") {
		parts := strings.SplitN(line, "\t", 3)
		if len(parts) < 3 {
			continue
		}
		acts = append(acts, Action{ID: parts[0], Cmd: parts[1], Name: parts[2], Group: "plugin"})
	}
	return acts, true
}
