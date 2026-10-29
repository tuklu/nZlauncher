package main

import (
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/tuklu/nZlauncher/internal/identity"
	"github.com/tuklu/nZlauncher/internal/shell"
)

// nz pick [-p prompt] [-i] — universal dmenu replacement: stdin lines in,
// selection out, rendered as fzf in a floating ghostty window.
// -i (input mode) returns the typed query when nothing matches.
func runPick(args []string) error {
	prompt := "Pick"
	inputMode := false
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-p":
			if i+1 < len(args) {
				i++
				prompt = args[i]
			}
		case "-i":
			inputMode = true
		}
	}

	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		return err
	}

	fzfArgs := []string{
		"--layout", "reverse", "--info", "hidden", "--no-separator",
		"--prompt", prompt + "… ", "--pointer", "▌", "--highlight-line",
	}
	if inputMode {
		fzfArgs = append(fzfArgs, "--print-query", "--bind", acceptTyped)
	}

	lines, err := floatFzf(string(data), fzfArgs, identity.PickClass)
	if err != nil {
		return nil
	}

	result := ""
	if inputMode {
		// --print-query: first line is the query, second the selection (if any)
		if len(lines) > 1 && lines[1] != "" {
			result = lines[1]
		} else if len(lines) > 0 {
			result = lines[0]
		}
	} else if len(lines) > 0 {
		result = lines[0]
	}

	if result != "" {
		os.Stdout.WriteString(result + "\n")
	}
	return nil
}

var errCancelled = errors.New("cancelled")

// Enter with nothing matching hands back what was typed, like dmenu.
const acceptTyped = "enter:accept-or-print-query"

// floatFzf runs fzf inside a floating ghostty window over the given input,
// blocking until the window closes, and returns fzf's output lines.
func floatFzf(input string, fzfArgs []string, class string) ([]string, error) {
	dir, err := os.MkdirTemp("", "nz-pick-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)

	inFile := filepath.Join(dir, "in")
	outFile := filepath.Join(dir, "out")
	if err := os.WriteFile(inFile, []byte(input), 0o600); err != nil {
		return nil, err
	}

	quoted := make([]string, len(fzfArgs))
	for i, a := range fzfArgs {
		quoted[i] = shell.Quote(a)
	}
	statusFile := filepath.Join(dir, "status")
	script := "fzf " + strings.Join(quoted, " ") +
		" < " + shell.Quote(inFile) + " > " + shell.Quote(outFile) +
		"; echo $? > " + shell.Quote(statusFile)

	cmd := exec.Command("ghostty", "--class="+class,
		"--gtk-single-instance=false",
		solidConfig(),
		"--window-padding-x=12", "--window-padding-y=10",
		"-e", "bash", "-c", script)
	cmd.Run()

	// 130 is Escape or Ctrl-C: with --print-query fzf still prints the query
	// then, so the status is the only way to tell a cancel from an answer
	if status, err := os.ReadFile(statusFile); err != nil || strings.TrimSpace(string(status)) == "130" {
		return nil, errCancelled
	}
	data, err := os.ReadFile(outFile)
	if err != nil || len(data) == 0 {
		return nil, nil
	}
	return strings.Split(strings.TrimRight(string(data), "\n"), "\n"), nil
}

// dmenu flags that take a value; their values are look and placement, which
// the floating fzf window decides itself.
var dmenuValueFlags = map[string]bool{
	"-l": true, "-fn": true, "-nb": true, "-nf": true, "-sb": true, "-sf": true,
	"-m": true, "-w": true, "--placeholder": true,
}

// runDmenu is a drop-in for dmenu (and the dmenu modes of rofi and walker):
// -p sets the prompt, Enter prints the selection or, with no match, what was
// typed, and cancelling exits 1 the way scripts expect. Appearance flags are
// accepted and ignored; matching is already case-insensitive.
func runDmenu(args []string) error {
	prompt := ""
	for i := 0; i < len(args); i++ {
		switch a := args[i]; {
		case a == "-p":
			if i+1 < len(args) {
				i++
				prompt = args[i]
			}
		case dmenuValueFlags[a]:
			i++
		}
	}

	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		return err
	}
	if prompt == "" {
		prompt = "Pick"
	}
	fzfArgs := []string{
		"--layout", "reverse", "--info", "hidden", "--no-separator",
		"--prompt", prompt + "… ", "--pointer", "▌", "--highlight-line",
		"--print-query", "--bind", acceptTyped,
	}
	lines, err := floatFzf(string(data), fzfArgs, identity.PickClass)
	if err != nil {
		os.Exit(1)
	}

	// --print-query: the query first, then the selection if there was one
	result := ""
	if len(lines) > 1 && lines[1] != "" {
		result = lines[1]
	} else if len(lines) > 0 {
		result = lines[0]
	}
	if result == "" {
		os.Exit(1)
	}
	os.Stdout.WriteString(result + "\n")
	return nil
}
