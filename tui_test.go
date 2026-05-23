package main

import (
	"strings"
	"testing"

	"github.com/jixishi/SerialTerminalForWindowsTerminal/internal/event"
)

func TestParseCtrlKey(t *testing.T) {
	tests := []struct {
		in     string
		want   byte
		ok     bool
		reason string
	}{
		{in: "ctrl+c", want: 'c', ok: true, reason: "plain ctrl"},
		{in: "ctrl+shift+c", ok: false, reason: "ctrl+shift reserved for local"},
		{in: "ctrl+enter", ok: false, reason: "non-letter"},
		{in: "alt+c", ok: false, reason: "wrong modifier"},
	}

	for _, tt := range tests {
		got, ok := parseCtrlKey(tt.in)
		if ok != tt.ok || got != tt.want {
			t.Fatalf("%s parseCtrlKey(%q) got=(%q,%v) want=(%q,%v)", tt.reason, tt.in, got, ok, tt.want, tt.ok)
		}
	}
}

func TestRenderModal(t *testing.T) {
	modal := renderModal("Title", "line1\nline2", 80)
	if !strings.Contains(modal, "Title") {
		t.Fatalf("renderModal missing title: %q", modal)
	}
	if !strings.Contains(modal, "line1") || !strings.Contains(modal, "line2") {
		t.Fatalf("renderModal missing lines: %q", modal)
	}
	if !strings.Contains(modal, "╭") || !strings.Contains(modal, "╮") || !strings.Contains(modal, "╰") || !strings.Contains(modal, "╯") {
		t.Fatalf("renderModal missing box borders: %q", modal)
	}
}

func TestHandleCtrlShiftLocalHelp(t *testing.T) {
	a := &App{uiEvents: make(chan event.UIEvent, 4), cfg: &Config{HotkeyMod: "ctrl+alt"}}
	a.SetUIEnabled(true)
	m := uiModel{app: a}

	ok := handleLocalHotkey(&m, "ctrl+alt+h")
	if !ok {
		t.Fatalf("expected local hotkey to be handled")
	}

	ev := mustReadEvent(t, a.uiEvents)
	if ev.Kind != event.UIEventModal {
		t.Fatalf("expected modal event, got %+v", ev)
	}
}

func TestNormalizeHotkeyPrefix(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"", "ctrl+alt"},
		{"ctrl+alt", "ctrl+alt"},
		{"ctrl+shift", "ctrl+shift"},
		{"CTRL+ALT", "ctrl+alt"},
		{"  ctrl+SHIFT  ", "ctrl+shift"},
		{"invalid", "ctrl+alt"},
	}

	for _, tt := range tests {
		got := normalizeHotkeyPrefix(tt.in)
		if got != tt.want {
			t.Fatalf("normalizeHotkeyPrefix(%q) got=%q want=%q", tt.in, got, tt.want)
		}
	}
}

func TestHotkeyWith(t *testing.T) {
	got := hotkeyWith("ctrl+alt", "h")
	if got != "ctrl+alt+h" {
		t.Fatalf("hotkeyWith ctrl+alt+h got=%q", got)
	}
	got = hotkeyWith("ctrl+shift", "c")
	if got != "ctrl+shift+c" {
		t.Fatalf("hotkeyWith ctrl+shift+c got=%q", got)
	}
}

func TestIsLocalHotkeyAll(t *testing.T) {
	tests := []struct {
		key, mod string
		action   string
		want     bool
	}{
		{"ctrl+alt+c", "ctrl+alt", "c", true},
		{"ctrl+shift+c", "ctrl+shift", "c", true},
		{"ctrl+alt+c", "ctrl+shift", "c", false},
		{"ctrl+shift+c", "ctrl+alt", "c", false},
		{"alt+c", "ctrl+alt", "c", false},
		{"ctrl+c", "ctrl+alt", "c", false},
	}

	for _, tt := range tests {
		a := &App{cfg: &Config{HotkeyMod: tt.mod}}
		m := uiModel{app: a}
		got := m.isLocalHotkey(tt.key, tt.action)
		if got != tt.want {
			t.Fatalf("isLocalHotkey(%q, %q) hotkeyMod=%q got=%v want=%v", tt.key, tt.action, tt.mod, got, tt.want)
		}
	}
}

func TestParseCtrlKeyEdgeCases(t *testing.T) {
	tests := []struct {
		in   string
		want byte
		ok   bool
	}{
		{in: "ctrl+z", want: 'z', ok: true},
		{in: "ctrl+a", want: 'a', ok: true},
		{in: "ctrl+shift+c", want: 0, ok: false},
		{in: "ctrl+alt+c", want: 0, ok: false},
		{in: "ctrl+", want: 0, ok: false},
		{in: "ctrl+ab", want: 0, ok: false},
		{in: "ctrl+A", want: 0, ok: false},
		{in: "ctrl+1", want: 0, ok: false},
	}

	for _, tt := range tests {
		got, ok := parseCtrlKey(tt.in)
		if ok != tt.ok || got != tt.want {
			t.Fatalf("parseCtrlKey(%q) got=(%q,%v) want=(%q,%v)", tt.in, got, ok, tt.want, tt.ok)
		}
	}
}

func TestRenderModalLongContent(t *testing.T) {
	longBody := "line1\nline2\nline3\nline4\nline5\nline6\nline7\nline8\nline9\nline10\nline11\nline12\nline13\nline14"
	modal := renderModal("Title", longBody, 80)
	if !strings.Contains(modal, "... (press Esc/Enter to close)") {
		t.Fatalf("long modal should be truncated: %q", modal)
	}
	if strings.Contains(modal, "line14") {
		t.Fatalf("line14 should not appear in truncated modal")
	}
}

func TestRenderModalEmpty(t *testing.T) {
	modal := renderModal("", "", 80)
	if !strings.Contains(modal, "Info") {
		t.Fatalf("empty title should default to Info: %q", modal)
	}
}

func TestTruncateToWidth(t *testing.T) {
	tests := []struct {
		in    string
		width int
		want  string
	}{
		{"hello", 3, "hel"},
		{"hello", 10, "hello"},
		{"", 5, ""},
		{"hello", 0, "hello"},
	}

	for _, tt := range tests {
		got := truncateToWidth(tt.in, tt.width)
		if got != tt.want {
			t.Fatalf("truncateToWidth(%q, %d) got=%q want=%q", tt.in, tt.width, got, tt.want)
		}
	}
}

func TestClampIndex(t *testing.T) {
	tests := []struct {
		idx, n int
		want   int
	}{
		{2, 5, 2},
		{-1, 5, 0},
		{10, 5, 4},
		{0, 0, 0},
		{0, 1, 0},
	}

	for _, tt := range tests {
		got := clampIndex(tt.idx, tt.n)
		if got != tt.want {
			t.Fatalf("clampIndex(%d, %d) got=%d want=%d", tt.idx, tt.n, got, tt.want)
		}
	}
}

func TestMinInt(t *testing.T) {
	if got := minInt(1, 2); got != 1 {
		t.Fatalf("minInt(1,2) got=%d", got)
	}
	if got := minInt(5, 3); got != 3 {
		t.Fatalf("minInt(5,3) got=%d", got)
	}
	if got := minInt(0, 0); got != 0 {
		t.Fatalf("minInt(0,0) got=%d", got)
	}
}

func TestMaxIntFunc(t *testing.T) {
	if got := maxInt(1, 2); got != 2 {
		t.Fatalf("maxInt(1,2) got=%d", got)
	}
	if got := maxInt(5, 3, 7); got != 7 {
		t.Fatalf("maxInt(5,3,7) got=%d", got)
	}
}

func TestHandleLocalHotkeyForward(t *testing.T) {
	a := &App{uiEvents: make(chan event.UIEvent, 4), cfg: &Config{HotkeyMod: "ctrl+alt"}}
	a.SetUIEnabled(true)
	m := uiModel{app: a}

	if !handleLocalHotkey(&m, "ctrl+alt+f") {
		t.Fatalf("expected forward hotkey handled")
	}
	ev := mustReadEvent(t, a.uiEvents)
	if ev.Kind != event.UIEventPanel || ev.Panel != event.UIPanelForward {
		t.Fatalf("expected forward panel, got %+v", ev)
	}
}

func TestHandleLocalHotkeyPlugin(t *testing.T) {
	a := &App{uiEvents: make(chan event.UIEvent, 4), cfg: &Config{HotkeyMod: "ctrl+alt"}}
	a.SetUIEnabled(true)
	m := uiModel{app: a}

	if !handleLocalHotkey(&m, "ctrl+alt+p") {
		t.Fatalf("expected plugin hotkey handled")
	}
	ev := mustReadEvent(t, a.uiEvents)
	if ev.Kind != event.UIEventPanel || ev.Panel != event.UIPanelPlugin {
		t.Fatalf("expected plugin panel, got %+v", ev)
	}
}

func TestHandleLocalHotkeyMode(t *testing.T) {
	a := &App{uiEvents: make(chan event.UIEvent, 4), cfg: &Config{HotkeyMod: "ctrl+alt"}}
	a.SetUIEnabled(true)
	m := uiModel{app: a}

	if !handleLocalHotkey(&m, "ctrl+alt+m") {
		t.Fatalf("expected mode hotkey handled")
	}
	ev := mustReadEvent(t, a.uiEvents)
	if ev.Kind != event.UIEventPanel || ev.Panel != event.UIPanelMode {
		t.Fatalf("expected mode panel, got %+v", ev)
	}
}

func TestHandleLocalHotkeyUnknown(t *testing.T) {
	a := &App{cfg: &Config{HotkeyMod: "ctrl+alt"}}
	m := uiModel{app: a}

	if handleLocalHotkey(&m, "ctrl+alt+x") {
		t.Fatalf("unknown hotkey should not be handled")
	}
}

func TestHandleLocalHotkeyCtrlShift(t *testing.T) {
	a := &App{uiEvents: make(chan event.UIEvent, 4), cfg: &Config{HotkeyMod: "ctrl+shift"}}
	a.SetUIEnabled(true)
	m := uiModel{app: a}

	if !handleLocalHotkey(&m, "ctrl+shift+h") {
		t.Fatalf("expected ctrl+shift+h to be handled")
	}
	ev := mustReadEvent(t, a.uiEvents)
	if ev.Kind != event.UIEventModal {
		t.Fatalf("expected help modal with ctrl+shift+h")
	}
}

func TestRenderPanelModal(t *testing.T) {
	lines := []panelLine{
		{text: "Header", selected: false},
		{text: "Selected Row", selected: true},
	}
	out := renderPanelModal("Test Panel", lines, "Footer text", 80)
	if !strings.Contains(out, "Test Panel") {
		t.Fatalf("missing title: %q", out)
	}
	if !strings.Contains(out, "Header") {
		t.Fatalf("missing header line: %q", out)
	}
	if !strings.Contains(out, "Selected Row") {
		t.Fatalf("missing selected line: %q", out)
	}
	if !strings.Contains(out, "Footer text") {
		t.Fatalf("missing footer: %q", out)
	}
}

func TestStyleFunctions(t *testing.T) {
	_ = modalFooterLineStyle()
	rendered := selectedPanelLineStyle().Render("test")
	if !strings.Contains(rendered, "test") {
		t.Fatalf("selectedPanelLineStyle should render text")
	}
}
