package main

import (
	"fmt"
	"os"
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

var config Config

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
