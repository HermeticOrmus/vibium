//go:build windows

package main

import "os"

func disableEcho(f *os.File) func() {
	return func() {}
}
