package main

import (
	"fmt"

	"github.com/BurntSushi/toml"
)

type Action struct {
	ID       string   `toml:"id"`
	Name     string   `toml:"name"`
	Icon     string   `toml:"icon"`
	Group    string   `toml:"group"`
	Cmd      string   `toml:"cmd"`
	Keywords []string `toml:"keywords"`
}

func (a Action) display() string {
	if a.Icon != "" {
		return a.Icon + "  " + a.Name
	}
	return a.Name
}

func loadCatalog(path string) ([]Action, error) {
	var c struct {
		Action []Action `toml:"action"`
	}
	if _, err := toml.DecodeFile(path, &c); err != nil {
		return nil, fmt.Errorf("catalog: %w", err)
	}
	for i, a := range c.Action {
		if a.ID == "" || a.Name == "" || a.Cmd == "" {
			return nil, fmt.Errorf("catalog: action %d missing id, name, or cmd", i)
		}
	}
	return c.Action, nil
}
