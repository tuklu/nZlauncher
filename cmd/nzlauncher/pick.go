package main

import (
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
		fzfArgs = append(fzfArgs, "--print-query")
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
	script := "fzf " + strings.Join(quoted, " ") +
		" < " + shell.Quote(inFile) + " > " + shell.Quote(outFile)

	cmd := exec.Command("ghostty", "--class="+class,
		"--gtk-single-instance=false",
		"--window-padding-x=12", "--window-padding-y=10",
		"-e", "bash", "-c", script)
	cmd.Run() // non-zero exit just means dismissed

	data, err := os.ReadFile(outFile)
	if err != nil || len(data) == 0 {
		return nil, nil
	}
	return strings.Split(strings.TrimRight(string(data), "\n"), "\n"), nil
}
