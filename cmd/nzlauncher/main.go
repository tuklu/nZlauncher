// nzlauncher is the CLI: launcher faces, the engine as TSV, and helpers.
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/tuklu/nZlauncher/internal/config"
	"github.com/tuklu/nZlauncher/internal/engine"
	"github.com/tuklu/nZlauncher/internal/hypr"
	"github.com/tuklu/nZlauncher/internal/server"
	"github.com/tuklu/nZlauncher/internal/shell"
	"github.com/tuklu/nZlauncher/internal/ui/gtk"
	"github.com/tuklu/nZlauncher/internal/ui/tui"
)

func main() {
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
	case "pick":
		err = runPick(os.Args[2:])
	case "gtk":
		err = runGTK(strings.Join(os.Args[2:], " "))
	case "serve":
		err = server.Serve()
	case "focus":
		if len(os.Args) < 3 {
			usage()
		}
		err = hypr.Focus(os.Args[2])
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
	fmt.Fprintln(os.Stderr, "usage: nzlauncher bar [query] | tui [query] | gtk [query] | pick | query [text] | serve | focus <address>")
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
	catalog, _ := config.Load()
	return catalog.Settings
}

func runGTK(initial string) error {
	return gtk.Run(initial, loadSettings().GTKMode())
}
