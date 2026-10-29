package providers

import (
	"fmt"
	"github.com/tuklu/nZlauncher/internal/compositor"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
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
	if !On("windows") {
		return nil
	}
	clients, err := compositor.Windows()
	if err != nil {
		return nil
	}

	// a window shares its process with others in single-instance apps
	// (one Ghostty process for every terminal), and then what runs inside
	// cannot be told apart per window: only the title says
	pids := map[int]int{}
	for _, c := range clients {
		pids[c.PID]++
	}

	var acts []action.Action
	for _, c := range clients {
		if c.Title == "" || strings.HasPrefix(c.Class, identity.AppID) {
			continue
		}
		keywords := append([]string{c.Class, "window", "switch", "focus"}, titleAliases(c.Title)...)
		if c.PID > 0 && pids[c.PID] == 1 {
			keywords = append(keywords, innerPrograms(c.PID)...)
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
			Keywords: keywords,
			Class:    c.Class,
			Hint:     workspaceHint(c.Workspace),
			Cmd:      "nzlauncher focus " + shell.Quote(c.ID),
		})
	}
	return acts
}

func workspaceHint(space string) string {
	if space == "" {
		return "window"
	}
	return "workspace " + space
}

// aliases are what people call what shows in a title: "yt" for YouTube.
var aliases = map[string][]string{
	"youtube": {"yt"}, "github": {"gh"}, "gitlab": {"gl"}, "whatsapp": {"wa"},
	"nvim": {"neovim", "vim", "editor"}, "neovim": {"nvim", "vim", "editor"},
	"visual studio code": {"vscode", "code"}, "telegram": {"tg"}, "instagram": {"ig", "insta"},
	"twitter": {"x"}, "chatgpt": {"gpt"}, "google docs": {"docs"}, "google meet": {"meet"},
}

func titleAliases(title string) []string {
	lower := strings.ToLower(title)
	var out []string
	for name, short := range aliases {
		if strings.Contains(lower, name) {
			out = append(out, short...)
		}
	}
	return out
}

// innerPrograms names what runs under a process, a few levels down: the
// nvim, htop or ssh inside a terminal window.
func innerPrograms(pid int) []string {
	var names []string
	var walk func(pid, depth int)
	walk = func(pid, depth int) {
		if depth > 4 || len(names) > 16 {
			return
		}
		tasks, _ := filepath.Glob(fmt.Sprintf("/proc/%d/task/*/children", pid))
		for _, t := range tasks {
			data, err := os.ReadFile(t)
			if err != nil {
				continue
			}
			for _, field := range strings.Fields(string(data)) {
				child, err := strconv.Atoi(field)
				if err != nil {
					continue
				}
				if comm, err := os.ReadFile(fmt.Sprintf("/proc/%d/comm", child)); err == nil {
					names = append(names, strings.TrimSpace(string(comm)))
				}
				walk(child, depth+1)
			}
		}
	}
	walk(pid, 0)
	return names
}

// kill [name] — user processes sorted by CPU, filtered by name.
func Kill(q string) []action.Action {
	if !On("kill") {
		return nil
	}
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
	if !On("wallpaper") {
		return nil
	}
	q = strings.ToLower(q)
	settings, _ := config.LoadSettings()
	dir, setCmd := settings.Wallpapers()
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
