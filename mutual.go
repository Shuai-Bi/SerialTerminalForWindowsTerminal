package main

import (
	"io"
	"os"

	"github.com/jixishi/SerialTerminalForWindowsTerminal/internal/session"
)

var (
	sess *session.SerialSession
	out  io.Writer = os.Stdout
)
