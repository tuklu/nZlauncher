package providers

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/tuklu/nZlauncher/internal/action"
	"github.com/tuklu/nZlauncher/internal/config"
)

// Community providers: drop an executable in ~/.config/nzlauncher/providers.d/.
// Its filename is the trigger word; it is invoked with the rest of the query
// as $1 and prints one action per line:
//
//	id <TAB> shell-command <TAB> display text
func Plugin(word, q string) ([]action.Action, bool) {
	if !On("plugins") {
		return nil, false
	}
	path := filepath.Join(config.Dir(), "providers.d", word)
	info, err := os.Stat(path)
	if err != nil || info.IsDir() || info.Mode()&0o111 == 0 {
		return nil, false
	}

	out, err := exec.Command(path, q).Output()
	if err != nil {
		return nil, true
	}

	var acts []action.Action
	for _, line := range strings.Split(strings.TrimRight(string(out), "\n"), "\n") {
		parts := strings.SplitN(line, "\t", 3)
		if len(parts) < 3 {
			continue
		}
		acts = append(acts, action.Action{ID: parts[0], Cmd: parts[1], Name: parts[2], Group: "plugin"})
	}
	return acts, true
}
