//go:build windows

package termapp

import (
	"golang.org/x/sys/windows"
)

func enableVTInput(fd int) {
	var mode uint32
	if err := windows.GetConsoleMode(windows.Handle(fd), &mode); err == nil {
		_ = windows.SetConsoleMode(windows.Handle(fd), mode|windows.ENABLE_VIRTUAL_TERMINAL_INPUT)
	}
}
