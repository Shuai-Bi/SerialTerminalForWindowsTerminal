package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/jixishi/SerialTerminalForWindowsTerminal/internal/event"
	"github.com/jixishi/SerialTerminalForWindowsTerminal/pkg/charset"
	"github.com/jixishi/SerialTerminalForWindowsTerminal/pkg/forward"
	"github.com/jixishi/SerialTerminalForWindowsTerminal/pkg/luaplugin"
)

type App struct {
	cfg        *Config
	forward    *forward.Manager
	plugins    *luaplugin.Manager
	dispatcher *CommandDispatcher

	uiEvents chan event.UIEvent
	done     chan struct{}

	stdinMu    sync.Mutex
	closeOnce  sync.Once
	closedFlag atomic.Bool
	uiEnabled  atomic.Bool

	logFile *os.File
}

func NewApp(cfg *Config) (*App, error) {
	f, err := openLogFile()
	if err != nil {
		return nil, err
	}

	a := &App{
		cfg:      cfg,
		plugins:  luaplugin.NewManager(),
		uiEvents: make(chan event.UIEvent, 512),
		done:     make(chan struct{}),
		logFile:  f,
	}
	a.uiEnabled.Store(true)

	a.forward = forward.NewManager(a.writeRawToSession, a.Notifyf)
	a.forward.SetInboundReporter(a.reportForwardIngress)
	a.dispatcher = NewCommandDispatcher(a)
	if err = a.loadDefaultDemoPlugin(); err != nil {
		return nil, err
	}
	return a, nil
}

func (a *App) loadDefaultDemoPlugin() error {
	demoPath := filepath.Join("plugins", "demo.lua")
	if _, err := os.Stat(demoPath); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	name, err := a.plugins.Load(demoPath)
	if err != nil {
		return err
	}
	return a.plugins.Disable(name)
}

func (a *App) Notifyf(format string, args ...any) {
	a.emit(event.UIEvent{Kind: event.UIEventOutput, Text: fmt.Sprintf(format, args...)})
}

func (a *App) Statusf(format string, args ...any) {
	a.emit(event.UIEvent{Kind: event.UIEventStatus, Text: fmt.Sprintf(format, args...)})
}

func (a *App) ShowModal(title, text string) {
	a.emit(event.UIEvent{Kind: event.UIEventModal, Title: title, Text: text})
}

func (a *App) OpenPanel(panel event.UIPanelKind) {
	a.emit(event.UIEvent{Kind: event.UIEventPanel, Panel: panel})
}

func (a *App) SetUIEnabled(enabled bool) {
	a.uiEnabled.Store(enabled)
}

func (a *App) UIEnabled() bool {
	return a.uiEnabled.Load()
}

func (a *App) emit(ev event.UIEvent) {
	if ev.Kind != event.UIEventPanel && ev.Text == "" {
		return
	}

	if !a.UIEnabled() {
		switch ev.Kind {
		case event.UIEventOutput:
			_, _ = io.WriteString(out, ev.Text)
		case event.UIEventStatus:
			_, _ = io.WriteString(out, ev.Text)
			if !strings.HasSuffix(ev.Text, "\n") {
				_, _ = io.WriteString(out, "\n")
			}
		case event.UIEventModal:
			_, _ = io.WriteString(out, "\n["+ev.Title+"]\n"+ev.Text+"\n")
		}
		if ev.Kind == event.UIEventOutput {
			a.appendLog(ev.Text)
		}
		return
	}

	select {
	case a.uiEvents <- ev:
	default:
		// Keep UI responsive; drop oldest when overloaded.
		select {
		case <-a.uiEvents:
		default:
		}
		a.uiEvents <- ev
	}

	if ev.Kind == event.UIEventOutput {
		a.appendLog(ev.Text)
	}
}

func (a *App) appendLog(text string) {
	if a.logFile == nil {
		return
	}

	_, _ = a.logFile.WriteString(text)
}

func (a *App) isClosed() bool {
	return a.closedFlag.Load()
}

func (a *App) Close() {
	a.closeOnce.Do(func() {
		a.closedFlag.Store(true)
		close(a.done)
		a.forward.Close()
		a.plugins.Close()
		CloseTrzsz()
		CloseSerial()
		if a.logFile != nil {
			_ = a.logFile.Close()
		}
	})
}

func (a *App) waitDone() <-chan struct{} {
	return a.done
}

func (a *App) loadConfiguredForwards() {
	for i, mode := range config.forWard {
		m := forward.Mode(mode)
		if m == forward.None {
			continue
		}
		if i >= len(config.address) {
			a.Notifyf("[forward] skip #%d: missing address", i)
			continue
		}
		addr := strings.TrimSpace(config.address[i])
		if addr == "" {
			continue
		}
		if _, err := a.forward.Add(m, addr); err != nil {
			a.Notifyf("[forward] add %s %s failed: %v", m.String(), addr, err)
		}
	}
}

func (a *App) reportForwardIngress(id int, chunk []byte) {
	if len(chunk) == 0 {
		return
	}

	if strings.EqualFold(a.cfg.inputCode, "hex") {
		a.Notifyf("[forward#%d -> serial] % X\n", id, chunk)
		return
	}

	converted, err := charset.ConvertChunk(chunk, a.cfg.inputCode, a.cfg.outputCode)
	if err != nil {
		converted = bytes.Clone(chunk)
	}
	text := string(converted)
	if !strings.HasSuffix(text, "\n") {
		text += "\n"
	}
	a.Notifyf("[forward#%d -> serial] %s", id, text)
}

func (a *App) writeRawToSession(data []byte) error {
	if len(data) == 0 {
		return nil
	}

	a.stdinMu.Lock()
	defer a.stdinMu.Unlock()
	_, err := stdinPipe.Write(data)
	return err
}

func (a *App) writeToSession(data []byte) error {
	processed, err := a.plugins.ProcessInput(data)
	if err != nil {
		return err
	}
	if len(processed) == 0 {
		return nil
	}

	return a.writeRawToSession(processed)
}

func (a *App) sendLine(line string) error {
	if strings.TrimSpace(line) == "" {
		return nil
	}

	payload := append([]byte(line), []byte(a.cfg.endStr)...)
	return a.writeToSession(payload)
}

func (a *App) sendCtrl(letter byte) error {
	if letter >= 'A' && letter <= 'Z' {
		letter = letter + ('a' - 'A')
	}
	control := []byte{letter & 0x1f}
	_, err := serialPort.Write(control)
	return err
}

func (a *App) handleLine(line string) {
	line = strings.TrimRight(line, "\r\n")
	if strings.TrimSpace(line) == "" {
		return
	}

	if strings.HasPrefix(strings.TrimSpace(line), ".") {
		next, allow, err := a.plugins.ProcessCommand(line)
		if err != nil {
			a.Notifyf("[plugin] command hook failed: %v", err)
			return
		}
		if !allow {
			a.Notifyf("[plugin] command blocked")
			return
		}
		if next != "" {
			line = next
		}
		handled, err := a.dispatcher.Execute(line)
		if err != nil {
			a.Statusf("[cmd] %v", err)
		}
		if handled {
			return
		}
	}

	if err := a.sendLine(line); err != nil {
		a.Statusf("[send] %v", err)
	}
}

func (a *App) startOutputLoop() {
	if strings.EqualFold(a.cfg.inputCode, "hex") {
		go a.readHexOutput()
		return
	}

	go a.readTextOutput()
}

func (a *App) readHexOutput() {
	frameSize := a.cfg.frameSize
	if frameSize <= 0 {
		frameSize = 16
	}

	buf := make([]byte, frameSize)
	for {
		n, err := stdoutPipe.Read(buf)
		if n > 0 {
			chunk := make([]byte, n)
			copy(chunk, buf[:n])
			a.forward.Broadcast(chunk)
			outChunk, hookErr := a.plugins.ProcessOutput(chunk)
			if hookErr != nil {
				a.Notifyf("[plugin] output hook failed: %v", hookErr)
				continue
			}
			if len(outChunk) == 0 {
				continue
			}
			a.emit(event.UIEvent{Kind: event.UIEventOutput, Text: charset.FormatHexFrame(outChunk, a.cfg.timesTamp, a.cfg.timesFmt)})
		}
		if err != nil {
			if err != io.EOF {
				a.Notifyf("[output] %v", err)
			}
			return
		}

		select {
		case <-a.done:
			return
		default:
		}
	}
}

func (a *App) readTextOutput() {
	buf := make([]byte, 4096)
	for {
		n, err := stdoutPipe.Read(buf)
		if n > 0 {
			chunk := make([]byte, n)
			copy(chunk, buf[:n])
			a.forward.Broadcast(chunk)

			outChunk, hookErr := a.plugins.ProcessOutput(chunk)
			if hookErr != nil {
				a.Notifyf("[plugin] output hook failed: %v", hookErr)
				continue
			}
			if len(outChunk) == 0 {
				continue
			}

			converted, convErr := charset.ConvertChunk(outChunk, a.cfg.inputCode, a.cfg.outputCode)
			if convErr != nil {
				a.Notifyf("[output] convert failed: %v", convErr)
				converted = bytes.Clone(outChunk)
			}

			text := string(converted)
			if a.cfg.timesTamp {
				text = prefixLines(text, time.Now().Format(a.cfg.timesFmt)+" ")
			}
			a.emit(event.UIEvent{Kind: event.UIEventOutput, Text: text})
		}
		if err != nil {
			if err != io.EOF {
				a.Notifyf("[output] %v", err)
			}
			return
		}

		select {
		case <-a.done:
			return
		default:
		}
	}
}

func prefixLines(s, prefix string) string {
	if s == "" || prefix == "" {
		return s
	}

	lines := strings.SplitAfter(s, "\n")
	for i, line := range lines {
		if line == "" {
			continue
		}
		lines[i] = prefix + line
	}
	return strings.Join(lines, "")
}
