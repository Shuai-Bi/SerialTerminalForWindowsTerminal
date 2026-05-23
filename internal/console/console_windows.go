//go:build windows

package console

import "golang.org/x/sys/windows"

func enableVTInput(fd int) {
	var mode uint32
	if err := windows.GetConsoleMode(windows.Handle(fd), &mode); err != nil {
		return
	}
	mode |= windows.ENABLE_VIRTUAL_TERMINAL_INPUT
	_ = windows.SetConsoleMode(windows.Handle(fd), mode)
}
