// Package app provides the core application coordinator.
package app

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

	appconfig "github.com/jixishi/SerialTerminalForWindowsTerminal/internal/config"
	"github.com/jixishi/SerialTerminalForWindowsTerminal/internal/event"
	"github.com/jixishi/SerialTerminalForWindowsTerminal/internal/session"
	"github.com/jixishi/SerialTerminalForWindowsTerminal/pkg/charset"
	"github.com/jixishi/SerialTerminalForWindowsTerminal/pkg/forward"
	"github.com/jixishi/SerialTerminalForWindowsTerminal/pkg/luaplugin"

	"github.com/jixishi/SerialTerminalForWindowsTerminal/internal/command"
)

// App is the central coordinator for the serial terminal application.
type App struct {
	cfg  *appconfig.Config
	sess *session.SerialSession
	out  io.Writer

	forward    *forward.Manager
	plugins    *luaplugin.Manager
	dispatcher *command.Dispatcher

	uiEvents chan event.UIEvent
	done     chan struct{}

	stdinMu    sync.Mutex
	closeOnce  sync.Once
	closedFlag atomic.Bool
	uiEnabled  atomic.Bool

	logFile *os.File
}

var _ command.CommandHost = (*App)(nil)

// New creates a new App with the given configuration, session, and output writer.
func New(cfg *appconfig.Config, sess *session.SerialSession, out io.Writer) (*App, error) {
	f, err := appconfig.OpenLogFile(cfg)
	if err != nil {
		return nil, err
	}

	a := &App{
		cfg:      cfg,
		sess:     sess,
		out:      out,
		plugins:  luaplugin.NewManager(),
		uiEvents: make(chan event.UIEvent, 512),
		done:     make(chan struct{}),
		logFile:  f,
	}
	a.uiEnabled.Store(true)

	a.forward = forward.NewManager(a.writeRawToSession, a.Notifyf)
	a.forward.SetInboundReporter(a.reportForwardIngress)
	a.dispatcher = command.NewDispatcher(a)
	if err = a.loadPluginsFromDir(); err != nil {
		return nil, err
	}
	return a, nil
}

// --- command.CommandHost implementation ---

func (a *App) Cfg() *appconfig.Config        { return a.cfg }
func (a *App) Forward() *forward.Manager     { return a.forward }
func (a *App) Plugins() *luaplugin.Manager   { return a.plugins }
func (a *App) WriteToSession(data []byte) error { return a.writeToSession(data) }

// --- exported accessors for TUI / console ---

func (a *App) UIEvents() <-chan event.UIEvent { return a.uiEvents }
func (a *App) WaitDone() <-chan struct{}       { return a.done }
func (a *App) SendCtrl(letter byte) error      { return a.sendCtrl(letter) }
func (a *App) HandleLine(line string)          { a.handleLine(line) }
func (a *App) Dispatcher() *command.Dispatcher  { return a.dispatcher }
func (a *App) StartOutputLoop()                { a.startOutputLoop() }
func (a *App) LoadConfiguredForwards()          { a.loadConfiguredForwards() }
func (a *App) Sess() *session.SerialSession    { return a.sess }
func (a *App) Out() io.Writer                  { return a.out }

func (a *App) loadPluginsFromDir() error {
	entries, err := os.ReadDir("plugins")
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".lua") {
			continue
		}
		pluginPath := filepath.Join("plugins", entry.Name())
		name, loadErr := a.plugins.Load(pluginPath)
		if loadErr != nil {
			a.Notifyf("[plugin] load %s failed: %v", entry.Name(), loadErr)
			continue
		}
		// Disable by default; user enables via .plugin enable or TUI panel
		_ = a.plugins.Disable(name)
	}
	return nil
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
			_, _ = io.WriteString(a.out, ev.Text)
		case event.UIEventStatus:
			_, _ = io.WriteString(a.out, ev.Text)
			if !strings.HasSuffix(ev.Text, "\n") {
				_, _ = io.WriteString(a.out, "\n")
			}
		case event.UIEventModal:
			_, _ = io.WriteString(a.out, "\n["+ev.Title+"]\n"+ev.Text+"\n")
		}
		if ev.Kind == event.UIEventOutput {
			a.appendLog(ev.Text)
		}
		return
	}

	select {
	case a.uiEvents <- ev:
	default:
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

func (a *App) Close() {
	a.closeOnce.Do(func() {
		a.closedFlag.Store(true)
		close(a.done)
		a.forward.Close()
		a.plugins.Close()
		if a.sess != nil {
			a.sess.Close()
		}
		if a.logFile != nil {
			_ = a.logFile.Close()
		}
	})
}

func (a *App) loadConfiguredForwards() {
	for i, mode := range a.cfg.ForWard {
		m := forward.Mode(mode)
		if m == forward.None {
			continue
		}
		if i >= len(a.cfg.Address) {
			a.Notifyf("[forward] skip #%d: missing address", i)
			continue
		}
		addr := strings.TrimSpace(a.cfg.Address[i])
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
	if strings.EqualFold(a.cfg.InputCode, "hex") {
		a.Notifyf("[forward#%d -> serial] % X\n", id, chunk)
		return
	}
	converted, err := charset.ConvertChunk(chunk, a.cfg.InputCode, a.cfg.OutputCode)
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
	_, err := a.sess.StdinPipe.Write(data)
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
	payload := append([]byte(line), []byte(a.cfg.EndStr)...)
	return a.writeToSession(payload)
}

func (a *App) sendCtrl(letter byte) error {
	if letter >= 'A' && letter <= 'Z' {
		letter = letter + ('a' - 'A')
	}
	control := []byte{letter & 0x1f}
	_, err := a.sess.Port.Write(control)
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
	if strings.EqualFold(a.cfg.InputCode, "hex") {
		go a.readHexOutput()
		return
	}
	go a.readTextOutput()
}

func (a *App) readHexOutput() {
	frameSize := a.cfg.FrameSize
	if frameSize <= 0 {
		frameSize = 16
	}

	buf := make([]byte, frameSize)
	for {
		n, err := a.sess.StdoutPipe.Read(buf)
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
			a.emit(event.UIEvent{Kind: event.UIEventOutput, Text: charset.FormatHexFrame(outChunk, a.cfg.TimesTamp, a.cfg.TimesFmt)})
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
		n, err := a.sess.StdoutPipe.Read(buf)
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

			converted, convErr := charset.ConvertChunk(outChunk, a.cfg.InputCode, a.cfg.OutputCode)
			if convErr != nil {
				a.Notifyf("[output] convert failed: %v", convErr)
				converted = bytes.Clone(outChunk)
			}

			text := string(converted)
			if a.cfg.TimesTamp {
				text = prefixLines(text, time.Now().Format(a.cfg.TimesFmt)+" ")
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
