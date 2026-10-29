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
	// Hint is the short right-hand column: a key combo, a size, a date.
	// Faces show the group when it is empty.
	Hint string `toml:"hint"`
	// Preview is a file a face may show beside the list: an image, or text.
	Preview string `toml:"-"`
	// Requires names programs the command needs besides its first word;
	// a catalog entry whose programs are not installed is hidden.
	Requires []string `toml:"requires"`
	// Unless hides an entry when one of these is installed: a portable
	// screenshot entry gives way to neZha's screenshot script.
	Unless []string `toml:"unless"`
}

// Badge is what goes in the right-hand column.
func (a Action) Badge() string {
	if a.Hint != "" {
		return a.Hint
	}
	return a.Group
}

func (a Action) Display() string {
	if a.Icon != "" {
		return a.Icon + "  " + a.Name
	}
	return a.Name
}
