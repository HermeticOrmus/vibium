//go:build !windows

package main

import (
	"os"
	"os/exec"
)

func disableEcho(f *os.File) func() {
	cmd := exec.Command("stty", "-echo")
	cmd.Stdin = f
	if err := cmd.Run(); err != nil {
		return func() {}
	}
	return func() {
		restore := exec.Command("stty", "echo")
		restore.Stdin = f
		_ = restore.Run()
	}
}
