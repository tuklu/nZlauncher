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
	// broken files still open the face; the engine reports the error in place
	settings, _ := config.LoadSettings()
	args := os.Args[1:]
	// --resident: keep running hidden between uses, the bar shows it again
	resident := len(args) > 0 && args[0] == "--resident"
	if resident {
		args = args[1:]
	}
	if err := gtk.Run(strings.Join(args, " "), settings.GTKMode(), resident); err != nil {
		fmt.Fprintln(os.Stderr, "nzlauncher-gtk:", err)
		os.Exit(1)
	}
}
