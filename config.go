package main

import (
	appconfig "github.com/jixishi/SerialTerminalForWindowsTerminal/internal/config"
)

// Config is an alias for appconfig.Config to keep main-package code concise.
type Config = appconfig.Config

var cfg = &Config{}
