//go:build windows

package main

import (
	"os"

	"golang.org/x/sys/windows"
)

// init enables ANSI escape sequences on consoles that do not enable them by
// default. Handles that are not consoles, such as pipes, are left untouched.
func init() {
	for _, f := range []*os.File{os.Stdout, os.Stderr} {
		handle := windows.Handle(f.Fd())
		var mode uint32
		if windows.GetConsoleMode(handle, &mode) == nil {
			_ = windows.SetConsoleMode(handle, mode|windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING)
		}
	}
}
