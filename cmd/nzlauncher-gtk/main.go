// nzlauncher-gtk is the GTK face. It is a binary of its own so that every
// other nzlauncher command starts without loading GTK.
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/tuklu/nZlauncher/internal/config"
	"github.com/tuklu/nZlauncher/internal/ui/gtk"
)

func main() {
	config.Migrate()
	// a broken catalog still opens the face; the engine reports the error in place
	catalog, _ := config.Load()
	if err := gtk.Run(strings.Join(os.Args[1:], " "), catalog.Settings.GTKMode()); err != nil {
		fmt.Fprintln(os.Stderr, "nzlauncher-gtk:", err)
		os.Exit(1)
	}
}
