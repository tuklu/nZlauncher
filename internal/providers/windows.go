package providers

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/tuklu/nZlauncher/internal/action"
	"github.com/tuklu/nZlauncher/internal/config"
	"github.com/tuklu/nZlauncher/internal/identity"
	"github.com/tuklu/nZlauncher/internal/shell"
)

// Open windows join the regular ranking pool: typing an app name offers
// "switch to it" alongside "launch another". Frecency keys on class, not
// address, so the habit of switching to an app survives window churn.
func Windows() []action.Action {
	out, err := exec.Command("hyprctl", "clients", "-j").Output()
	if err != nil {
		return nil
	}
	var clients []struct {
		Address string `json:"address"`
		Class   string `json:"class"`
		Title   string `json:"title"`
		Mapped  bool   `json:"mapped"`
	}
	if json.Unmarshal(out, &clients) != nil {
		return nil
	}

	var acts []action.Action
	for _, c := range clients {
		if !c.Mapped || c.Title == "" || strings.HasPrefix(c.Class, identity.AppID) {
			continue
		}
		title := []rune(c.Title)
		if len(title) > 60 {
			title = append(title[:57], []rune("...")...)
		}
		acts = append(acts, action.Action{
			ID:       "win:" + c.Class,
			Name:     string(title),
			Icon:     "󱂬",
			Group:    "window",
			Keywords: []string{c.Class, "window", "switch", "focus"},
			Cmd:      "hyprctl dispatch focuswindow address:" + c.Address,
		})
	}
	return acts
}

// kill [name] — user processes sorted by CPU, filtered by name.
func Kill(q string) []action.Action {
	q = strings.ToLower(q)
	out, err := exec.Command("ps", "-u", os.Getenv("USER"),
		"-o", "pid,pcpu,pmem,comm", "--sort=-pcpu", "--no-headers").Output()
	if err != nil {
		return nil
	}

	var acts []action.Action
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}
		pid, cpu, mem, comm := fields[0], fields[1], fields[2], fields[3]
		if q != "" && !strings.Contains(strings.ToLower(comm), q) {
			continue
		}
		acts = append(acts, action.Action{
			ID:    "kill:" + comm,
			Name:  fmt.Sprintf("Kill %s   (pid %s, cpu %s%%, mem %s%%)", comm, pid, cpu, mem),
			Icon:  "󰚌",
			Group: "kill",
			Cmd:   "kill " + pid,
		})
		if len(acts) == 25 {
			break
		}
	}
	return append(acts, tuiTail("btop", "Btop", "full process monitor")...)
}

// wallpaper [name]: images in the wallpaper_dir setting, applied with
// wallpaper_cmd and the image path appended.
func Wallpaper(q string) []action.Action {
	q = strings.ToLower(q)
	catalog, _ := config.Load()
	dir, setCmd := catalog.Settings.Wallpapers()
	if _, err := exec.LookPath(strings.Fields(setCmd)[0]); err != nil {
		return nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	var acts []action.Action
	for _, e := range entries {
		name := e.Name()
		ext := strings.ToLower(filepath.Ext(name))
		if strings.HasPrefix(name, ".") || ext != ".jpg" && ext != ".jpeg" && ext != ".png" && ext != ".webp" {
			continue
		}
		if q != "" && !strings.Contains(strings.ToLower(name), q) {
			continue
		}
		acts = append(acts, action.Action{
			ID:    "wp:" + name,
			Name:  "Wallpaper: " + name,
			Icon:  "󰸉",
			Group: "wallpaper",
			Cmd:   setCmd + " " + shell.Quote(filepath.Join(dir, name)),
		})
	}
	return acts
}
