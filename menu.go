package main

import (
	"os"
	"os/exec"
	"sort"
	"strings"
	"syscall"
	"time"
)

func runMenu(group string) error {
	actions, err := loadCatalog(catalogPath())
	if err != nil {
		return err
	}

	if group == "" {
		actions = append(actions, scanApps()...)
	} else {
		filtered := actions[:0]
		for _, a := range actions {
			if a.Group == group {
				filtered = append(filtered, a)
			}
		}
		actions = filtered
		if group == "app" {
			actions = append(actions, scanApps()...)
		}
	}

	stats := loadStats()
	now := time.Now().Unix()
	sort.SliceStable(actions, func(i, j int) bool {
		si, sj := score(stats[actions[i].ID], now), score(stats[actions[j].ID], now)
		if si != sj {
			return si > sj
		}
		return actions[i].Name < actions[j].Name
	})

	lines := make([]string, 0, len(actions))
	byDisplay := make(map[string]Action, len(actions))
	for _, a := range actions {
		d := a.display()
		if _, dup := byDisplay[d]; dup {
			continue
		}
		lines = append(lines, d)
		byDisplay[d] = a
	}

	prompt := "Run"
	if group != "" {
		prompt = strings.ToUpper(group[:1]) + group[1:]
	}

	choice, err := pick(lines, prompt)
	if err != nil || choice == "" {
		return err
	}

	action, ok := byDisplay[choice]
	if !ok {
		return nil
	}

	if err := recordLaunch(action.ID, ""); err != nil {
		return err
	}
	return launch(action.Cmd)
}

func pick(lines []string, prompt string) (string, error) {
	out, err := floatFzf(strings.Join(lines, "\n"), []string{
		"--layout", "reverse", "--info", "hidden", "--no-separator",
		"--prompt", prompt + "… ", "--pointer", "▌", "--highlight-line",
	}, "nz.pick")
	if err != nil || len(out) == 0 {
		return "", nil
	}
	return strings.TrimSpace(out[0]), nil
}

func launch(command string) error {
	return startDetached(exec.Command("sh", "-c", command))
}

func startDetached(cmd *exec.Cmd) error {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	devnull, err := os.OpenFile(os.DevNull, os.O_RDWR, 0)
	if err == nil {
		cmd.Stdin, cmd.Stdout, cmd.Stderr = devnull, devnull, devnull
	}
	return cmd.Start()
}
