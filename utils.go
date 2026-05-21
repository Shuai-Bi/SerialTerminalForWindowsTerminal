package main

import (
	"fmt"
	"github.com/trzsz/trzsz-go/trzsz"
	"go.bug.st/serial"
	"golang.org/x/term"
	"io"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"sync"
)

func checkPortAvailability(name string) ([]string, error) {
	ports, err := serial.GetPortsList()
	if err != nil {
		return nil, err
	}
	if len(ports) == 0 {
		return nil, fmt.Errorf("无串口")
	}
	if name == "" {
		return ports, fmt.Errorf("串口未指定")
	}
	for _, port := range ports {
		if strings.Compare(port, name) == 0 {
			return ports, nil
		}
	}
	return ports, fmt.Errorf("串口 " + name + " 未在线")
}

func OpenSerial() error {
	mode := &serial.Mode{
		BaudRate: config.baudRate,
		StopBits: serial.StopBits(config.stopBits),
		DataBits: config.dataBits,
		Parity:   serial.Parity(config.parityBit),
	}
	var err error
	serialPort, err = serial.Open(config.portName, mode)
	return err
}

func CloseSerial() {
	if serialPort == nil {
		return
	}

	if err := serialPort.Close(); err != nil {
		fmt.Fprint(os.Stderr, err)
		fmt.Fprint(os.Stderr, "\n")
	}
}

var termch chan os.Signal
var termchOnce sync.Once

// OpenTrzsz create a TrzszFilter to support trzsz ( trz / tsz ).
//
// ┌────────┐   stdinPipe  ┌────────┐   ClientIn   ┌─────────────┐   SerialIn   ┌────────┐
// │        ├─────────────►│        ├─────────────►│             ├─────────────►│        │
// │ mutual │              │ Client │              │ TrzszFilter │              │ Serial │
// │        │◄─────────────│        │◄─────────────┤             │◄─────────────┤        │
// └────────┘   stdoutPipe └────────┘   ClientOut  └─────────────┘   SerialOut  └────────┘
func OpenTrzsz() error {
	fd := int(os.Stdin.Fd())
	width, _, err := term.GetSize(fd)
	if err != nil {
		if runtime.GOOS != "windows" {
			return fmt.Errorf("term get size failed: %w", err)
		}
		width = 80
	}

	clientIn, stdinPipe = io.Pipe()
	stdoutPipe, clientOut = io.Pipe()
	trzszFilter = trzsz.NewTrzszFilter(clientIn, clientOut, serialPort, serialPort,
		trzsz.TrzszOptions{TerminalColumns: int32(width), EnableZmodem: true})
	trzsz.SetAffectedByWindows(false)
	termch = make(chan os.Signal, 1)
	termchOnce = sync.Once{}

	go func() {
		for range termch {
			width, _, err := term.GetSize(fd)
			if err != nil {
				fmt.Printf("term get size failed: %s\n", err)
				continue
			}
			trzszFilter.SetTerminalColumns(int32(width))
		}
	}()

	return nil
}

func CloseTrzsz() {
	if termch == nil {
		return
	}

	termchOnce.Do(func() {
		signal.Stop(termch)
		close(termch)
	})
}

