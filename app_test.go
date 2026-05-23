package main

import (
	"io"
	"net"
	"testing"
	"time"

	"go.bug.st/serial"

	"github.com/jixishi/SerialTerminalForWindowsTerminal/internal/event"
	"github.com/jixishi/SerialTerminalForWindowsTerminal/internal/session"
	"github.com/jixishi/SerialTerminalForWindowsTerminal/pkg/forward"
	"github.com/jixishi/SerialTerminalForWindowsTerminal/pkg/luaplugin"
)

func TestPrefixLines(t *testing.T) {
	tests := []struct {
		name   string
		in     string
		prefix string
		want   string
	}{
		{name: "empty", in: "", prefix: "X ", want: ""},
		{name: "no-prefix", in: "a\n", prefix: "", want: "a\n"},
		{name: "single-line", in: "abc", prefix: "T ", want: "T abc"},
		{name: "multi-line", in: "a\nb\n", prefix: "P ", want: "P a\nP b\n"},
	}

	for _, tt := range tests {
		got := prefixLines(tt.in, tt.prefix)
		if got != tt.want {
			t.Fatalf("%s: prefixLines got=%q want=%q", tt.name, got, tt.want)
		}
	}
}

func TestAppUIEvents(t *testing.T) {
	a := &App{uiEvents: make(chan event.UIEvent, 8)}
	a.SetUIEnabled(true)

	a.Notifyf("hello %s", "world")
	a.Statusf("ok")
	a.ShowModal("Title", "Body")

	ev1 := mustReadEvent(t, a.uiEvents)
	if ev1.Kind != event.UIEventOutput || ev1.Text != "hello world" {
		t.Fatalf("unexpected output event: %+v", ev1)
	}

	ev2 := mustReadEvent(t, a.uiEvents)
	if ev2.Kind != event.UIEventStatus || ev2.Text != "ok" {
		t.Fatalf("unexpected status event: %+v", ev2)
	}

	ev3 := mustReadEvent(t, a.uiEvents)
	if ev3.Kind != event.UIEventModal || ev3.Title != "Title" || ev3.Text != "Body" {
		t.Fatalf("unexpected modal event: %+v", ev3)
	}
}

func TestSendLine(t *testing.T) {
	setupTestPipes()
	a := &App{
		cfg:      &Config{EndStr: "\r\n"},
		plugins:  luaplugin.NewManager(),
		uiEvents: make(chan event.UIEvent, 8),
		done:     make(chan struct{}),
	}
	a.SetUIEnabled(true)

	if err := a.sendLine("hello"); err != nil {
		t.Fatalf("sendLine failed: %v", err)
	}

	if err := a.sendLine(""); err != nil {
		t.Fatalf("sendLine empty string should be no-op: %v", err)
	}
	if err := a.sendLine("   "); err != nil {
		t.Fatalf("sendLine whitespace should be no-op: %v", err)
	}
}

func TestHandleLine(t *testing.T) {
	setupTestPipes()
	a := &App{
		cfg:      &Config{EndStr: "\n", InputCode: "UTF-8", OutputCode: "UTF-8"},
		plugins:  luaplugin.NewManager(),
		uiEvents: make(chan event.UIEvent, 8),
		done:     make(chan struct{}),
	}
	a.SetUIEnabled(true)
	a.forward = forward.NewManager(func([]byte) error { return nil }, func(string, ...any) {})
	a.dispatcher = NewCommandDispatcher(a)

	a.handleLine("hello")
	a.handleLine("")
	a.handleLine(".help")

	ev := mustReadEvent(t, a.uiEvents)
	if ev.Kind != event.UIEventModal || ev.Title == "" {
		t.Fatalf("expected .help modal, got %+v", ev)
	}
}

func TestEmitNonUI(t *testing.T) {
	oldOut := out
	out = io.Discard
	defer func() { out = oldOut }()

	a := &App{
		uiEvents: make(chan event.UIEvent, 4),
		logFile:  nil,
	}
	a.SetUIEnabled(false)

	a.emit(event.UIEvent{Kind: event.UIEventOutput, Text: "serial data\n"})
	a.emit(event.UIEvent{Kind: event.UIEventStatus, Text: "status msg"})
	a.emit(event.UIEvent{Kind: event.UIEventModal, Title: "T", Text: "body"})
	a.emit(event.UIEvent{Kind: event.UIEventOutput, Text: ""})
}

func TestEmitUISaturation(t *testing.T) {
	a := &App{
		uiEvents: make(chan event.UIEvent, 2),
	}
	a.SetUIEnabled(true)

	// Fill channel
	a.emit(event.UIEvent{Kind: event.UIEventOutput, Text: "a"})
	a.emit(event.UIEvent{Kind: event.UIEventOutput, Text: "b"})
	// This should drop oldest and insert newest
	a.emit(event.UIEvent{Kind: event.UIEventOutput, Text: "c"})

	ev := mustReadEvent(t, a.uiEvents)
	if ev.Text != "b" {
		t.Fatalf("expected b after drop, got %q", ev.Text)
	}
	ev = mustReadEvent(t, a.uiEvents)
	if ev.Text != "c" {
		t.Fatalf("expected c, got %q", ev.Text)
	}
}

func TestAppClose(t *testing.T) {
	a := &App{
		done:     make(chan struct{}),
		plugins:  luaplugin.NewManager(),
		forward:  forward.NewManager(func([]byte) error { return nil }, func(string, ...any) {}),
		uiEvents: make(chan event.UIEvent, 4),
	}
	a.SetUIEnabled(true)

	a.Close()
	if !a.isClosed() {
		t.Fatalf("expected app closed")
	}
	// Second close should be safe
	a.Close()
}

func TestLoadConfiguredForwards(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen failed: %v", err)
	}
	defer listener.Close()

	testCfg := &Config{
		ForWard: []int{int(forward.TCP), int(forward.None), int(forward.UDP)},
		Address: []string{listener.Addr().String(), "", ""},
	}

	a := &App{
		cfg:      testCfg,
		forward:  forward.NewManager(func([]byte) error { return nil }, func(string, ...any) {}),
		uiEvents: make(chan event.UIEvent, 8),
		done:     make(chan struct{}),
	}
	a.SetUIEnabled(true)

	a.loadConfiguredForwards()
	// forward.TCP should be added, forward.None skipped, forward.UDP skipped (empty address)
	items := a.forward.List()
	if len(items) != 1 || items[0].Mode != "tcp" {
		t.Fatalf("expected 1 TCP forward, got %+v", items)
	}
}

func TestReportForwardIngress(t *testing.T) {
	a := &App{
		cfg:      &Config{InputCode: "UTF-8", OutputCode: "UTF-8"},
		uiEvents: make(chan event.UIEvent, 4),
	}
	a.SetUIEnabled(true)

	a.reportForwardIngress(1, []byte("test"))

	// Hex mode
	a.cfg.InputCode = "hex"
	a.reportForwardIngress(2, []byte{0x41, 0x42})

	// Empty chunk
	a.reportForwardIngress(3, nil)
}

func TestSendCtrl(t *testing.T) {
	if sess == nil {
		sess = &session.SerialSession{}
	}
	oldSp := sess.Port
	defer func() { sess.Port = oldSp }()

	// Use a mock serial port
	sess.Port = &mockSerialPort{}
	a := &App{
		cfg:      &Config{},
		uiEvents: make(chan event.UIEvent, 4),
	}
	a.SetUIEnabled(true)

	if err := a.sendCtrl('c'); err != nil {
		t.Fatalf("sendCtrl('c') failed: %v", err)
	}
	if err := a.sendCtrl('C'); err != nil {
		t.Fatalf("sendCtrl('C') failed: %v", err)
	}
	if err := a.sendCtrl('A'); err != nil {
		t.Fatalf("sendCtrl('A') failed: %v", err)
	}
}

type mockSerialPort struct{}

func (m *mockSerialPort) Write(p []byte) (int, error)     { return len(p), nil }
func (m *mockSerialPort) Read(p []byte) (int, error)      { return 0, io.EOF }
func (m *mockSerialPort) Close() error                    { return nil }
func (m *mockSerialPort) SetMode(mode *serial.Mode) error { return nil }
func (m *mockSerialPort) SetDTR(dtr bool) error           { return nil }
func (m *mockSerialPort) SetRTS(rts bool) error           { return nil }
func (m *mockSerialPort) GetModemStatusBits() (*serial.ModemStatusBits, error) {
	return &serial.ModemStatusBits{}, nil
}
func (m *mockSerialPort) ResetInputBuffer() error              { return nil }
func (m *mockSerialPort) ResetOutputBuffer() error             { return nil }
func (m *mockSerialPort) SetReadTimeout(t time.Duration) error { return nil }
func (m *mockSerialPort) Break(t time.Duration) error          { return nil }
func (m *mockSerialPort) Drain() error                         { return nil }

func mustReadEvent(t *testing.T, ch <-chan event.UIEvent) event.UIEvent {
	t.Helper()
	select {
	case ev := <-ch:
		return ev
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for UI event")
		return event.UIEvent{}
	}
}
