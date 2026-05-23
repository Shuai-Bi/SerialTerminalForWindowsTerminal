package console

import (
	"testing"

	"github.com/jixishi/SerialTerminalForWindowsTerminal/internal/config"
)

func TestParseCSIu(t *testing.T) {
	tests := []struct {
		name string
		seq  []byte
		cp   int
		mod  int
		ok   bool
	}{
		{name: "ctrl+alt+c lowercase", seq: []byte{0x1b, '[', '9', '9', ';', '6', 'u'}, cp: 99, mod: 6, ok: true},
		{name: "ctrl+shift+c uppercase", seq: []byte{0x1b, '[', '6', '7', ';', '5', 'u'}, cp: 67, mod: 5, ok: true},
		{name: "too short", seq: []byte{0x1b, '[', '9', '9'}, cp: 0, mod: 0, ok: false},
		{name: "no escape prefix", seq: []byte{'[', '9', '9', ';', '6', 'u'}, cp: 0, mod: 0, ok: false},
		{name: "no u terminator", seq: []byte{0x1b, '[', '9', '9', ';', '6', 'x'}, cp: 0, mod: 0, ok: false},
		{name: "bad format no semicolon", seq: []byte{0x1b, '[', '9', '9', '6', 'u'}, cp: 0, mod: 0, ok: false},
		{name: "empty", seq: []byte{}, cp: 0, mod: 0, ok: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cp, mod, ok := parseCSIu(tt.seq)
			if ok != tt.ok || cp != tt.cp || mod != tt.mod {
				t.Fatalf("parseCSIu(%v) got=(%d,%d,%v) want=(%d,%d,%v)", tt.seq, cp, mod, ok, tt.cp, tt.mod, tt.ok)
			}
		})
	}
}

func TestIsExitHotkeySeq(t *testing.T) {
	cfg := &config.Config{HotkeyMod: "ctrl+alt"}

	// CSI u Ctrl+Alt+C (mod=6)
	if !isExitHotkeySeq([]byte{0x1b, '[', '9', '9', ';', '6', 'u'}, cfg) {
		t.Fatalf("Ctrl+Alt+C CSI should exit with ctrl+alt config")
	}
	if !isExitHotkeySeq([]byte{0x1b, '[', '9', '9', ';', '7', 'u'}, cfg) {
		t.Fatalf("Ctrl+Alt+Shift+C should also exit")
	}
	if isExitHotkeySeq([]byte{0x1b, '[', '9', '9', ';', '5', 'u'}, cfg) {
		t.Fatalf("Ctrl+Shift+C should NOT exit with ctrl+alt config")
	}
	if isExitHotkeySeq([]byte{0x1b, '[', '9', '7', ';', '6', 'u'}, cfg) {
		t.Fatalf("Ctrl+Alt+A should not exit")
	}
	if isExitHotkeySeq([]byte{0x1b, 'c'}, cfg) {
		t.Fatalf("Alt+C (ESC c) should NOT exit — Ctrl modifier required")
	}

	cfg2 := &config.Config{HotkeyMod: "ctrl+shift"}
	if !isExitHotkeySeq([]byte{0x1b, '[', '9', '9', ';', '5', 'u'}, cfg2) {
		t.Fatalf("Ctrl+Shift+C should exit with ctrl+shift config")
	}
	if !isExitHotkeySeq([]byte{0x1b, '[', '9', '9', ';', '7', 'u'}, cfg2) {
		t.Fatalf("Ctrl+Shift+Alt+C should also exit (includes Ctrl+Shift)")
	}
	if isExitHotkeySeq([]byte{0x1b, '[', '9', '9', ';', '6', 'u'}, cfg2) {
		t.Fatalf("Ctrl+Alt+C should NOT exit with ctrl+shift config")
	}
	if isExitHotkeySeq([]byte{0x1b, 'c'}, cfg2) {
		t.Fatalf("ESC c should NOT exit with ctrl+shift config")
	}
	if isExitHotkeySeq([]byte{0x1b, 'x'}, cfg2) {
		t.Fatalf("ESC x should not exit")
	}
	if isExitHotkeySeq([]byte("hello"), cfg2) {
		t.Fatalf("plain bytes should not exit")
	}

	cfg3 := &config.Config{HotkeyMod: "ctrl+alt"}
	if isExitHotkeySeq([]byte{0x1b, '[', '9', '9', ';', '4', 'u'}, cfg3) {
		t.Fatalf("Ctrl+C (without Alt) should not exit")
	}
	if isExitHotkeySeq([]byte{0x1b, '[', '9', '9', ';', '2', 'u'}, cfg3) {
		t.Fatalf("Alt+C (without Ctrl) should not exit")
	}
}
