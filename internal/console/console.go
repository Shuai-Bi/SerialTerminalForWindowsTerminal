// Package console provides the non-TUI console mode.
package console

import (
	"fmt"
	"io"
	"os"
	"os/signal"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"golang.org/x/term"

	apppkg "github.com/jixishi/SerialTerminalForWindowsTerminal/internal/app"
	"github.com/jixishi/SerialTerminalForWindowsTerminal/internal/config"
	"github.com/jixishi/SerialTerminalForWindowsTerminal/internal/flag"
	"github.com/jixishi/SerialTerminalForWindowsTerminal/internal/session"
	"github.com/jixishi/SerialTerminalForWindowsTerminal/internal/tui"
)

// Run parses flags, sets up the session and app, then runs TUI or console mode.
func Run() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(os.Stderr, "fatal: %v\n", r)
			os.Exit(1)
		}
	}()

	cfg := &config.Config{}
	flag.Init(cfg)
	flag.Normalize()
	flag.Parse()
	flag.Ext(cfg)
	if cfg.PortName == "" {
		flag.GetCliFlag(cfg)
	}

	ports, err := session.CheckPortAvailability(cfg.PortName)
	if err != nil {
		fmt.Println(err)
		flag.PrintUsage(ports)
		os.Exit(0)
	}

	sess, err := session.Open(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "open session failed: %v\n", err)
		os.Exit(1)
	}

	appInst, err := apppkg.New(cfg, sess, os.Stdout)
	if err != nil {
		fmt.Fprintf(os.Stderr, "create app failed: %v\n", err)
		os.Exit(1)
	}
	defer appInst.Close()

	appInst.LoadConfiguredForwards()
	appInst.StartOutputLoop()

	go forwardInterruptToRemote(appInst)
	appInst.SetUIEnabled(cfg.EnableGUI)

	if cfg.EnableGUI {
		model := tui.New(appInst)
		p := tea.NewProgram(model, tea.WithAltScreen(), tea.WithoutSignalHandler())
		enableVTInput(int(os.Stdin.Fd())) // Restore VT input for Ctrl+Alt+Key hotkeys
		if _, err = p.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "tui failed: %v\n", err)
			os.Exit(1)
		}
		return
	}

	if err = RunConsole(appInst); err != nil {
		fmt.Fprintf(os.Stderr, "console failed: %v\n", err)
		os.Exit(1)
	}
}

func forwardInterruptToRemote(appInst *apppkg.App) {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)
	defer signal.Stop(sigCh)

	for {
		select {
		case <-appInst.WaitDone():
			return
		case <-sigCh:
			if err := appInst.SendCtrl('c'); err != nil {
				appInst.Notifyf("[signal] interrupt pass-through failed: %v", err)
				continue
			}
			appInst.Notifyf("[signal] Ctrl+C forwarded to remote")
		}
	}
}

// RunConsole runs the non-TUI console mode.
func RunConsole(appInst *apppkg.App) error {
	fd := int(os.Stdin.Fd())
	isTerm := term.IsTerminal(fd)
	var oldState *term.State
	var err error
	if isTerm {
		enableVTInput(fd)
		oldState, err = term.MakeRaw(fd)
		if err != nil {
			return err
		}
		defer func() { _ = term.Restore(fd, oldState) }()
	}

	appInst.Notifyf("[console] non-gui mode, commands start with '.' at line start\n")
	appInst.Notifyf("[console] Ctrl+<Key> passes through to remote; .exit to exit")

	ch := make(chan byte, 1024)
	errCh := make(chan error, 1)
	go func() {
		buf := make([]byte, 256)
		for {
			n, rdErr := os.Stdin.Read(buf)
			if rdErr != nil {
				errCh <- rdErr
				return
			}
			for i := 0; i < n; i++ {
				ch <- buf[i]
			}
		}
	}()

	out := appInst.Out()
	cfg := appInst.Cfg()
	lineStart := true
	commandMode := false
	cmdBuf := make([]byte, 0, 128)

	tryRead := func() (byte, bool) {
		select {
		case b := <-ch:
			return b, true
		default:
			return 0, false
		}
	}

	readByte := func() (byte, error) {
		select {
		case <-appInst.WaitDone():
			return 0, io.EOF
		case rdErr := <-errCh:
			return 0, rdErr
		case b := <-ch:
			return b, nil
		}
	}

	flushESC := func(seq []byte) bool {
		if isExitHotkeySeq(seq, cfg) {
			appInst.Close()
			return true
		}
		if err = appInst.WriteToSession(seq); err != nil {
			appInst.Statusf("[send] %v", err)
		}
		return false
	}

	for {
		b, rdErr := readByte()
		if rdErr != nil {
			if rdErr == io.EOF {
				return nil
			}
			return rdErr
		}

		if b == 0x1b {
			escBuf := []byte{0x1b}
			for {
				nb, ok := tryRead()
				if !ok {
					if err = appInst.WriteToSession([]byte{0x1b}); err != nil {
						appInst.Statusf("[send] %v", err)
					}
					break
				}
				escBuf = append(escBuf, nb)
				if nb >= 0x40 && nb <= 0x7e {
					if flushESC(escBuf) {
						return nil
					}
					break
				}
				if len(escBuf) == 2 && escBuf[1] != '[' {
					if flushESC(escBuf) {
						return nil
					}
					break
				}
				if len(escBuf) > 16 {
					if err = appInst.WriteToSession(escBuf); err != nil {
						appInst.Statusf("[send] %v", err)
					}
					break
				}
			}
			continue
		}

		if b == 0x00 {
			if b2, ok := tryRead(); ok {
				if isAltKeyExit(b2, cfg) {
					appInst.Close()
					return nil
				}
				if err = appInst.WriteToSession([]byte{0x00, b2}); err != nil {
					appInst.Statusf("[send] %v", err)
				}
			} else {
				if err = appInst.WriteToSession([]byte{0x00}); err != nil {
					appInst.Statusf("[send] %v", err)
				}
			}
			if commandMode {
				lineStart = false
			}
			continue
		}

		if commandMode {
			switch b {
			case '\r', '\n':
				echoConsoleNewline(out)
				line := string(cmdBuf)
				if strings.TrimSpace(line) != "" {
					appInst.HandleLine(line)
				}
				commandMode = false
				cmdBuf = cmdBuf[:0]
				lineStart = true
			case 0x7f, 0x08:
				if len(cmdBuf) > 0 {
					cmdBuf = cmdBuf[:len(cmdBuf)-1]
					echoConsoleBackspace(out)
				}
			case 0x09:
				line, cands := appInst.Dispatcher().Complete(string(cmdBuf))
				if len(cands) == 1 {
					cmdBuf = append(cmdBuf[:0], line...)
					echoRedrawCommand(out, line)
				} else if len(cands) > 1 {
					echoConsoleNewline(out)
					appInst.Notifyf("%s", strings.Join(cands, "  "))
					echoConsoleByte(out, '.')
					echoConsoleString(out, string(cmdBuf[1:]))
				}
			default:
				cmdBuf = append(cmdBuf, b)
				echoConsoleByte(out, b)
			}
			continue
		}

		if lineStart && b == '.' {
			commandMode = true
			cmdBuf = append(cmdBuf[:0], b)
			echoConsoleByte(out, b)
			continue
		}

		if b == '\r' || b == '\n' {
			if err = appInst.WriteToSession([]byte(cfg.EndStr)); err != nil {
				appInst.Statusf("[send] %v", err)
			}
			lineStart = true
		} else {
			if err = appInst.WriteToSession([]byte{b}); err != nil {
				appInst.Statusf("[send] %v", err)
			}
			lineStart = false
		}
	}
}

func parseCSIu(seq []byte) (cp int, mod int, ok bool) {
	if len(seq) < 6 {
		return 0, 0, false
	}
	if seq[0] != 0x1b || seq[1] != '[' {
		return 0, 0, false
	}
	if seq[len(seq)-1] != 'u' {
		return 0, 0, false
	}
	inner := string(seq[2 : len(seq)-1])
	parts := strings.SplitN(inner, ";", 2)
	if len(parts) != 2 {
		return 0, 0, false
	}
	cp, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, false
	}
	mod, err = strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, false
	}
	return cp, mod, true
}

func isAltKeyExit(b byte, cfg *config.Config) bool {
	if normalizeHotkey(cfg.HotkeyMod) != "ctrl+alt" {
		return false
	}
	return b == 0x2e || b == 0x03 || b == 0x63 || b == 0x43
}

func isExitHotkeySeq(seq []byte, cfg *config.Config) bool {
	mod := normalizeHotkey(cfg.HotkeyMod)
	if cp, cmod, ok := parseCSIu(seq); ok {
		if cp != 'c' && cp != 'C' {
			return false
		}
		switch mod {
		case "ctrl+alt":
			return cmod&6 == 6
		case "ctrl+shift":
			return cmod&5 == 5
		}
		return false
	}
	return false
}

func normalizeHotkey(mod string) string { return config.NormalizeHotkey(mod) }

func echoConsoleByte(out io.Writer, b byte)        { _, _ = out.Write([]byte{b}) }
func echoConsoleNewline(out io.Writer)              { _, _ = io.WriteString(out, "\r\n") }
func echoConsoleBackspace(out io.Writer)            { _, _ = io.WriteString(out, "\b \b") }
func echoConsoleString(out io.Writer, s string)     { _, _ = io.WriteString(out, s) }
func echoRedrawCommand(out io.Writer, s string)     { _, _ = io.WriteString(out, "\r\033[K> "+s) }
