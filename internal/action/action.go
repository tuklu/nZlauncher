// Package action defines the unit every provider produces and every face renders.
package action

type Action struct {
	ID       string   `toml:"id"`
	Name     string   `toml:"name"`
	Icon     string   `toml:"icon"`
	IconName string   `toml:"icon_name"`
	Group    string   `toml:"group"`
	Cmd      string   `toml:"cmd"`
	Keywords []string `toml:"keywords"`
}

func (a Action) Display() string {
	if a.Icon != "" {
		return a.Icon + "  " + a.Name
	}
	return a.Name
}
