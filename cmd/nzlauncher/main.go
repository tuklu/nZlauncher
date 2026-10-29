// nzlauncher is the CLI: launcher faces, the engine as TSV, and helpers.
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/tuklu/nZlauncher/internal/config"
	"github.com/tuklu/nZlauncher/internal/engine"
	"github.com/tuklu/nZlauncher/internal/hypr"
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
		usage()
	}
	config.Migrate()

	var err error
	switch os.Args[1] {
	case "query":
		err = runQuery(strings.Join(os.Args[2:], " "))
	case "tui":
		err = tui.Run(strings.Join(os.Args[2:], " "))
	case "bar":
		err = runBar(strings.Join(os.Args[2:], " "))
	case "dmenu":
		err = runDmenu(os.Args[2:])
	case "pick":
		err = runPick(os.Args[2:])
	case "gtk":
		err = runGTK(os.Args[2:])
	case "serve":
		err = server.Serve()
	case "focus":
		if len(os.Args) < 3 {
			usage()
		}
		err = hypr.Focus(os.Args[2])
	case "setup":
		err = runSetup()
	case "wifi":
		err = runWifi(os.Args[2:])
	case "after-close":
		err = shell.RunAfterClose(os.Args[2:])
	default:
		usage()
	}

	if err != nil {
		fmt.Fprintln(os.Stderr, "nzlauncher:", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: nzlauncher bar [query] | tui [query] | gtk [query] | pick | dmenu | query [text] | serve | setup | wifi connect|disconnect [ssid] | focus <address>")
	os.Exit(2)
}

// The resident daemon answers when it runs; otherwise the engine runs here.
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
