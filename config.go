package main

import (
	"fmt"
	"os"
	"strings"
	"time"
)

type Config struct {
	portName    string
	baudRate    int
	dataBits    int
	stopBits    int
	parityBit   int
	outputCode  string
	inputCode   string
	endStr      string
	enableLog   bool
	logFilePath string
	forWard     []int
	frameSize   int
	timesTamp   bool
	timesFmt    string
	address     []string
	enableGUI   bool
	hotkeyMod   string
}

type FoeWardMode int

const (
	NOT FoeWardMode = iota
	TCPC
	UDPC
)

var config Config

func (m FoeWardMode) Network() string {
	switch m {
	case TCPC:
		return "tcp"
	case UDPC:
		return "udp"
	default:
		return ""
	}
}

func (m FoeWardMode) String() string {
	switch m {
	case TCPC:
		return "tcp"
	case UDPC:
		return "udp"
	default:
		return "none"
	}
}

func parseForwardMode(v string) (FoeWardMode, bool) {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "tcp", "tcp-c", "tcpc", "1":
		return TCPC, true
	case "udp", "udp-c", "udpc", "2":
		return UDPC, true
	default:
		return NOT, false
	}
}

func openLogFile() (*os.File, error) {
	if config.enableLog {
		path := fmt.Sprintf(config.logFilePath, config.portName, time.Now().Format("2006_01_02T150405"))
		f, err := os.OpenFile(path, os.O_RDWR|os.O_APPEND|os.O_CREATE, 0666)
		if err != nil {
			return nil, err
		}
		return f, nil
	}

	return nil, nil
}
