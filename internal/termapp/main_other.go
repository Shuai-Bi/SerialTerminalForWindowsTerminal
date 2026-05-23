//go:build !windows

package termapp

func enableVTInput(fd int) {}
