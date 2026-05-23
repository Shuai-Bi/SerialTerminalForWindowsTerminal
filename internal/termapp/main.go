package termapp

import (
	"fmt"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/pflag"
	"io"
	"log"
	"os"
	"os/signal"
	"strconv"
	"strings"

	"github.com/jixishi/SerialTerminalForWindowsTerminal/internal/flag"
	"github.com/jixishi/SerialTerminalForWindowsTerminal/internal/session"
	"golang.org/x/term"
)

func init() {
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile | log.Lmsgprefix)
	flag.Init(cfg)
}

func Run() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(os.Stderr, "fatal: %v\n", r)
			os.Exit(1)
		}
	}()

	flag.Normalize()
	pflag.Parse()
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

	sess, err = session.Open(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "open session failed: %v\n", err)
		os.Exit(1)
	}

	app, err := NewApp(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "create app failed: %v\n", err)
		os.Exit(1)
	}
	defer app.Close()

	app.loadConfiguredForwards()
	app.startOutputLoop()

	go forwardInterruptToRemote(app)
	app.SetUIEnabled(cfg.EnableGUI)

	if cfg.EnableGUI {
		model := newUIModel(app)
		p := tea.NewProgram(model, tea.WithAltScreen(), tea.WithoutSignalHandler())
		if _, err = p.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "tui failed: %v\n", err)
			os.Exit(1)
		}
		return
	}

	if err = runConsole(app); err != nil {
		fmt.Fprintf(os.Stderr, "console failed: %v\n", err)
		os.Exit(1)
	}
}

func forwardInterruptToRemote(app *App) {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)
	defer signal.Stop(sigCh)

	for {
		select {
		case <-app.waitDone():
			return
		case <-sigCh:
			if err := app.sendCtrl('c'); err != nil {
				app.Notifyf("[signal] interrupt pass-through failed: %v", err)
				continue
			}
			app.Notifyf("[signal] Ctrl+C forwarded to remote")
		}
	}
}

func runConsole(app *App) error {
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
		defer func() {
			_ = term.Restore(fd, oldState)
		}()
	}

	app.Notifyf("[console] non-gui mode, commands start with '.' at line start\n")
	app.Notifyf("[console] Ctrl+<Key> passes through to remote; .exit to exit")

	// Read with a larger buffer so multi-byte sequences (arrows, CSI) arrive together.
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
		case <-app.waitDone():
			return 0, io.EOF
		case rdErr := <-errCh:
			return 0, rdErr
		case b := <-ch:
			return b, nil
		}
	}

	// flushESC sends a fully-built escape sequence to serial.
	flushESC := func(seq []byte) bool {
		if isExitHotkeySeq(seq) {
			app.Close()
			return true
		}
		if err = app.writeToSession(seq); err != nil {
			app.Statusf("[send] %v", err)
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

		// ── Escape sequences (VT / CSI) ──
		if b == 0x1b {
			// Try to read the rest without blocking.
			escBuf := []byte{0x1b}
			for {
				nb, ok := tryRead()
				if !ok {
					// Standalone ESC — send it now.
					if err = app.writeToSession([]byte{0x1b}); err != nil {
						app.Statusf("[send] %v", err)
					}
					break
				}
				escBuf = append(escBuf, nb)
				// CSI terminator byte (0x40–0x7E): A–Z, a–z, ~, etc.
				if nb >= 0x40 && nb <= 0x7e {
					if flushESC(escBuf) {
						return nil
					}
					break
				}
				// Short non-CSI sequence (e.g. ESC c).
				if len(escBuf) == 2 && escBuf[1] != '[' {
					if flushESC(escBuf) {
						return nil
					}
					break
				}
				// CSI parameter bytes (digits, semicolons, etc.) — keep collecting.
				if len(escBuf) > 16 {
					// Too long, just flush.
					if err = app.writeToSession(escBuf); err != nil {
						app.Statusf("[send] %v", err)
					}
					break
				}
			}
			continue
		}

		// ── Windows Alt+key: NULL prefix ──
		if b == 0x00 {
			if b2, ok := tryRead(); ok {
				if isAltKeyExit(b2) {
					app.Close()
					return nil
				}
				if err = app.writeToSession([]byte{0x00, b2}); err != nil {
					app.Statusf("[send] %v", err)
				}
			} else {
				// No second byte available — send NULL alone.
				if err = app.writeToSession([]byte{0x00}); err != nil {
					app.Statusf("[send] %v", err)
				}
			}
			if commandMode {
				lineStart = false
			}
			continue
		}

		// ── Command mode ──
		if commandMode {
			switch b {
			case '\r', '\n':
				echoConsoleNewline()
				line := string(cmdBuf)
				if strings.TrimSpace(line) != "" {
					app.handleLine(line)
				}
				commandMode = false
				cmdBuf = cmdBuf[:0]
				lineStart = true
			case 0x7f, 0x08:
				if len(cmdBuf) > 0 {
					cmdBuf = cmdBuf[:len(cmdBuf)-1]
					echoConsoleBackspace()
				}
			case 0x09: // Tab — command completion
				line, cands := app.dispatcher.Complete(string(cmdBuf))
				if len(cands) == 1 {
					cmdBuf = append(cmdBuf[:0], line...)
					echoRedrawCommand(line)
				} else if len(cands) > 1 {
					echoConsoleNewline()
					app.Notifyf("%s", strings.Join(cands, "  "))
					echoConsoleByte('.')
					echoConsoleString(string(cmdBuf[1:]))
				}
			default:
				cmdBuf = append(cmdBuf, b)
				echoConsoleByte(b)
			}
			continue
		}

		// ── Normal mode (sending to remote) ──
		if lineStart && b == '.' {
			commandMode = true
			cmdBuf = append(cmdBuf[:0], b)
			echoConsoleByte(b)
			continue
		}

		if b == '\r' || b == '\n' {
			if err = app.writeToSession([]byte(cfg.EndStr)); err != nil {
				app.Statusf("[send] %v", err)
			}
			lineStart = true
		} else {
			if err = app.writeToSession([]byte{b}); err != nil {
				app.Statusf("[send] %v", err)
			}
			lineStart = false
		}
	}
}

func parseCSIu(seq []byte) (cp int, mod int, ok bool) {
	// ESC [ codepoint ; modifier u
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

func isAltKeyExit(b byte) bool {
	if normalizeHotkeyPrefix(cfg.HotkeyMod) != "ctrl+alt" {
		return false
	}
	// 0x2E = scan code for 'C', 0x03 = Ctrl+C, 0x63 = 'c', 0x43 = 'C'
	return b == 0x2e || b == 0x03 || b == 0x63 || b == 0x43
}

func isExitHotkeySeq(seq []byte) bool {
	mod := normalizeHotkeyPrefix(cfg.HotkeyMod)

	// CSI u format: ESC [ codepoint ; modifier u
	// Only matches when the Ctrl modifier bit (4) is present,
	// distinguishing Ctrl+Alt+C from Alt+C alone.
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

func echoConsoleByte(b byte) {
	_, _ = out.Write([]byte{b})
}

func echoConsoleNewline() {
	_, _ = io.WriteString(out, "\r\n")
}

func echoConsoleBackspace() {
	_, _ = io.WriteString(out, "\b \b")
}

func echoConsoleString(s string) {
	_, _ = io.WriteString(out, s)
}

func echoRedrawCommand(s string) {
	_, _ = io.WriteString(out, "\r\033[K> "+s)
}
