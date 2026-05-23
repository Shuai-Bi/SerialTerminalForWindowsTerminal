// Package session manages the serial port connection and its associated pipes.
package session

import (
	"fmt"
	"io"
	"os"
	"os/signal"
	"runtime"
	"sync"

	"github.com/trzsz/trzsz-go/trzsz"
	"go.bug.st/serial"
	"golang.org/x/term"

	"github.com/jixishi/SerialTerminalForWindowsTerminal/internal/config"
)

// SerialSession owns the serial port, trzsz filter, and pipe pair.
type SerialSession struct {
	Port        serial.Port
	TrzszFilter *trzsz.TrzszFilter
	StdinPipe   *io.PipeWriter
	StdoutPipe  *io.PipeReader
	ClientIn    *io.PipeReader
	ClientOut   *io.PipeWriter

	termCh    chan os.Signal
	closeOnce sync.Once
}

// Open creates a SerialSession by opening the serial port and initializing trzsz.
func Open(cfg *config.Config) (*SerialSession, error) {
	mode := &serial.Mode{
		BaudRate: cfg.BaudRate,
		StopBits: serial.StopBits(cfg.StopBits),
		DataBits: cfg.DataBits,
		Parity:   serial.Parity(cfg.ParityBit),
	}
	port, err := serial.Open(cfg.PortName, mode)
	if err != nil {
		return nil, err
	}

	fd := int(os.Stdin.Fd())
	width, _, err := term.GetSize(fd)
	if err != nil {
		if runtime.GOOS != "windows" {
			port.Close()
			return nil, fmt.Errorf("term get size failed: %w", err)
		}
		width = 80
	}

	clientIn, stdinPipe := io.Pipe()
	stdoutPipe, clientOut := io.Pipe()
	trzszFilter := trzsz.NewTrzszFilter(clientIn, clientOut, port, port,
		trzsz.TrzszOptions{TerminalColumns: int32(width), EnableZmodem: true})
	trzsz.SetAffectedByWindows(false)

	s := &SerialSession{
		Port:        port,
		TrzszFilter: trzszFilter,
		StdinPipe:   stdinPipe,
		StdoutPipe:  stdoutPipe,
		ClientIn:    clientIn,
		ClientOut:   clientOut,
		termCh:      make(chan os.Signal, 1),
	}

	go func() {
		for range s.termCh {
			w, _, err := term.GetSize(fd)
			if err != nil {
				fmt.Printf("term get size failed: %s\n", err)
				continue
			}
			trzszFilter.SetTerminalColumns(int32(w))
		}
	}()

	return s, nil
}

// Write writes data to the stdin pipe (toward serial port, through trzsz).
func (s *SerialSession) Write(data []byte) (int, error) {
	return s.StdinPipe.Write(data)
}

// Read reads data from the stdout pipe (from serial port, through trzsz).
func (s *SerialSession) Read(buf []byte) (int, error) {
	return s.StdoutPipe.Read(buf)
}

// SendCtrl sends a control character directly to the serial port (bypasses trzsz).
func (s *SerialSession) SendCtrl(letter byte) (int, error) {
	if letter >= 'A' && letter <= 'Z' {
		letter = letter + ('a' - 'A')
	}
	control := []byte{letter & 0x1f}
	return s.Port.Write(control)
}

// Close tears down the session: stops term signals, closes trzsz, then serial port.
func (s *SerialSession) Close() {
	s.closeOnce.Do(func() {
		if s.termCh != nil {
			signal.Stop(s.termCh)
			close(s.termCh)
		}
		if s.Port != nil {
			if err := s.Port.Close(); err != nil {
				fmt.Fprint(os.Stderr, err)
				fmt.Fprint(os.Stderr, "\n")
			}
		}
	})
}

// CheckPortAvailability returns the list of available ports and verifies the named port exists.
func CheckPortAvailability(name string) ([]string, error) {
	ports, err := serial.GetPortsList()
	if err != nil {
		return nil, err
	}
	if len(ports) == 0 {
		return nil, fmt.Errorf("no serial ports found")
	}
	if name == "" {
		return ports, fmt.Errorf("port name not specified")
	}
	for _, port := range ports {
		if port == name {
			return ports, nil
		}
	}
	return ports, fmt.Errorf("port " + name + " is not available")
}
