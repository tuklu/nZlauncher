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
	case "query":
		err = runQuery(strings.Join(os.Args[2:], " "))
	case "tui":
		err = runTUI(strings.Join(os.Args[2:], " "))
	case "bar":
		err = runBar(strings.Join(os.Args[2:], " "))
	case "pick":
		err = runPick(os.Args[2:])
	case "gtk":
		err = runGTK(strings.Join(os.Args[2:], " "))
	case "after-close":
		err = runAfterClose(os.Args[2:])
	default:
		usage()
	}

	if err != nil {
		fmt.Fprintln(os.Stderr, "nz:", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: nz bar [query] | tui [query] | gtk [query] | pick | query [text]")
	os.Exit(2)
}

func catalogPath() string {
	return filepath.Join(os.Getenv("HOME"), ".config/neZha/catalog.toml")
}

func frecencyPath() string {
	return filepath.Join(os.Getenv("HOME"), ".local/share/neZha/frecency.json")
}
