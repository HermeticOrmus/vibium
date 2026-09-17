//go:build windows

package main

import (
	"os"

	"golang.org/x/sys/windows"
)

// disableEcho hides typed input on the Windows console so pasted API keys
// stay off the screen, matching the Unix stty path. A non-console handle
// (redirected stdin) leaves echo alone, like a failed stty.
func disableEcho(f *os.File) func() {
	handle := windows.Handle(f.Fd())
	var mode uint32
	if err := windows.GetConsoleMode(handle, &mode); err != nil {
		return func() {}
	}
	if err := windows.SetConsoleMode(handle, mode&^windows.ENABLE_ECHO_INPUT); err != nil {
		return func() {}
	}
	return func() {
		_ = windows.SetConsoleMode(handle, mode)
	}
}
