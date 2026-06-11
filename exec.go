package main

import (
	"os"
	"os/exec"
	"syscall"
)

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
