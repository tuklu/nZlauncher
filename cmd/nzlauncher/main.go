// nzlauncher is the CLI: launcher faces, the engine as TSV, and helpers.
package main

import (
	"fmt"
	"github.com/tuklu/nZlauncher/internal/compositor"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/tuklu/nZlauncher/internal/config"
	"github.com/tuklu/nZlauncher/internal/engine"
	"github.com/tuklu/nZlauncher/internal/server"
	"github.com/tuklu/nZlauncher/internal/shell"
	"github.com/tuklu/nZlauncher/internal/ui/tui"
)

func main() {
	// linked as dmenu, it is dmenu
	if filepath.Base(os.Args[0]) == "dmenu" {
		if err := runDmenu(os.Args[1:]); err != nil {
			fmt.Fprintln(os.Stderr, "dmenu:", err)
			os.Exit(1)
		}
		return
	}
	if len(os.Args) < 2 {
		printHelp(os.Stderr, "")
		os.Exit(2)
	}
	command, args := os.Args[1], os.Args[2:]
	switch command {
	case "-h", "--help":
		printHelp(os.Stdout, "")
		return
	case "-V", "--version":
		fmt.Println("nzlauncher", versionString())
		return
	}
	if _, known := commandHelp[command]; known && command != "help" && wantsHelp(args) {
		printHelp(os.Stdout, command)
		return
	}
	args = withoutDashDash(args)
	// help, man and completion run at package time, with no config to touch
	if command != "help" && command != "man" && command != "completion" {
		config.Migrate()
	}

	var err error
	switch command {
	case "query":
		err = runQuery(strings.Join(args, " "))
	case "tui":
		err = tui.Run(strings.Join(args, " "))
	case "bar":
		err = runBar(strings.Join(args, " "))
	case "dmenu":
		err = runDmenu(args)
	case "pick":
		err = runPick(args)
	case "gtk":
		err = runGTK(args)
	case "serve":
		err = noArgs("serve", args, server.Serve)
	case "focus":
		if len(args) != 1 {
			err = misuse("focus", "focus takes one window")
			break
		}
		err = compositor.Focus(args[0])
	case "ws":
		err = runWorkspace(args)
	case "setup":
		err = noArgs("setup", args, runSetup)
	case "wifi":
		err = runWifi(args)
	case "tweak":
		err = runTweak(args)
	case "catalog":
		err = runCatalog(args)
	case "help":
		err = runHelp(args)
	case "man":
		err = runMan(args)
	case "completion":
		err = runCompletion(args)
	case "after-close":
		// internal: the faces hand a command to run once they have closed
		err = shell.RunAfterClose(args)
	default:
		fmt.Fprintf(os.Stderr, "nzlauncher: no command %q\nTry 'nzlauncher --help'.\n", command)
		os.Exit(2)
	}

	if reportUsage(err) {
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "nzlauncher:", err)
		os.Exit(1)
	}
}

func noArgs(command string, args []string, run func() error) error {
	if len(args) != 0 {
		return misuse(command, "%s takes no arguments", command)
	}
	return run()
}

func runQuery(raw string) error {
	resp, err := server.Ask(server.Request{Op: "query", Q: raw, Focused: os.Getenv("NZ_FOCUSED")})
	if err != nil && resp.Error != "" {
		return err
	}
	if err != nil {
		acts, err := engine.Query(raw)
		if err != nil {
			return err
		}
		resp.Results = make([]server.Result, len(acts))
		for i, a := range acts {
			resp.Results[i] = server.Result{ID: a.ID, Cmd: a.Cmd, Display: a.Display()}
		}
	}
	clean := strings.NewReplacer("\t", " ", "\n", " ")
	for _, r := range resp.Results {
		fmt.Printf("%s\t%s\t%s\n", clean.Replace(r.ID), clean.Replace(r.Cmd), clean.Replace(r.Display))
	}
	return nil
}

// A broken catalog still opens a face; the engine reports the error in place.
func loadSettings() config.Settings {
	settings, _ := config.LoadSettings()
	return settings
}

// The GTK face lives in nzlauncher-gtk, next to this binary or on PATH.
func gtkBinary() (string, error) {
	if self, err := os.Executable(); err == nil {
		sibling := filepath.Join(filepath.Dir(self), "nzlauncher-gtk")
		if _, err := os.Stat(sibling); err == nil {
			return sibling, nil
		}
	}
	return exec.LookPath("nzlauncher-gtk")
}

func runGTK(args []string) error {
	bin, err := gtkBinary()
	if err != nil {
		return fmt.Errorf("the GTK face needs nzlauncher-gtk: %w", err)
	}
	return syscall.Exec(bin, append([]string{bin}, args...), os.Environ())
}

// runWorkspace switches to a workspace, or with "move" sends the focused
// window there.
func runWorkspace(args []string) error {
	move := len(args) == 2 && args[0] == "move"
	if move {
		args = args[1:]
	}
	if len(args) != 1 {
		return misuse("ws", "ws takes a workspace number")
	}
	n, err := strconv.Atoi(args[0])
	if err != nil || n < 1 {
		return fmt.Errorf("no workspace %q", args[0])
	}
	if move {
		return compositor.MoveToWorkspace(n)
	}
	return compositor.Workspace(n)
}
