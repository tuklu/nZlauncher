package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	if len(os.Args) < 2 {
		usage()
	}

	var err error
	switch os.Args[1] {
	case "menu":
		group := ""
		if len(os.Args) > 2 {
			group = os.Args[2]
		}
		err = runMenu(group)
	case "query":
		err = runQuery(strings.Join(os.Args[2:], " "))
	case "tui":
		err = runTUI()
	case "bar":
		err = runBar()
	default:
		usage()
	}

	if err != nil {
		fmt.Fprintln(os.Stderr, "nz:", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: nz bar | tui | query [text] | menu [group]")
	os.Exit(2)
}

func catalogPath() string {
	return filepath.Join(os.Getenv("HOME"), ".config/neZha/catalog.toml")
}

func frecencyPath() string {
	return filepath.Join(os.Getenv("HOME"), ".local/share/neZha/frecency.json")
}
