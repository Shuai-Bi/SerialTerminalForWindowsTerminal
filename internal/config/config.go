// Package config holds the application configuration.
package config

import (
	"fmt"
	"os"
	"strings"
	"time"
)

// Config holds all application settings.
type Config struct {
	PortName    string
	BaudRate    int
	DataBits    int
	StopBits    int
	ParityBit   int
	OutputCode  string
	InputCode   string
	EndStr      string
	EnableLog   bool
	LogFilePath string
	ForWard     []int
	FrameSize   int
	TimesTamp   bool
	TimesFmt    string
	Address     []string
	EnableGUI   bool
	HotkeyMod   string
}

// NormalizeHotkey validates and normalizes a hotkey modifier string.
func NormalizeHotkey(mod string) string {
	mod = strings.ToLower(strings.TrimSpace(mod))
	if mod != "ctrl+alt" && mod != "ctrl+shift" {
		mod = "ctrl+alt"
	}
	return mod
}

// OpenLogFile opens the configured log file for writing, or returns nil if logging is disabled.
func OpenLogFile(cfg *Config) (*os.File, error) {
	if cfg.EnableLog {
		path := fmt.Sprintf(cfg.LogFilePath, cfg.PortName, time.Now().Format("2006_01_02T150405"))
		f, err := os.OpenFile(path, os.O_RDWR|os.O_APPEND|os.O_CREATE, 0666)
		if err != nil {
			return nil, err
		}
		return f, nil
	}
	return nil, nil
}
