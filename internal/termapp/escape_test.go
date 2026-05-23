package termapp

import (
	"testing"
)

func TestParseCSIu(t *testing.T) {
	tests := []struct {
		name string
		seq  []byte
		cp   int
		mod  int
		ok   bool
	}{
		{
			name: "ctrl+alt+c lowercase",
			seq:  []byte{0x1b, '[', '9', '9', ';', '6', 'u'},
			cp:   99, mod: 6, ok: true,
		},
		{
			name: "ctrl+shift+c uppercase",
			seq:  []byte{0x1b, '[', '6', '7', ';', '5', 'u'},
			cp:   67, mod: 5, ok: true,
		},
		{
			name: "too short",
			seq:  []byte{0x1b, '[', '9', '9'},
			cp:   0, mod: 0, ok: false,
		},
		{
			name: "no escape prefix",
			seq:  []byte{'[', '9', '9', ';', '6', 'u'},
			cp:   0, mod: 0, ok: false,
		},
		{
			name: "no u terminator",
			seq:  []byte{0x1b, '[', '9', '9', ';', '6', 'x'},
			cp:   0, mod: 0, ok: false,
		},
		{
			name: "bad format no semicolon",
			seq:  []byte{0x1b, '[', '9', '9', '6', 'u'},
			cp:   0, mod: 0, ok: false,
		},
		{
			name: "empty",
			seq:  []byte{},
			cp:   0, mod: 0, ok: false,
		},
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
	oldCfg := *cfg
	defer func() { *cfg = oldCfg }()

	*cfg = Config{HotkeyMod: "ctrl+alt"}

	// CSI u Ctrl+Alt+C (mod=6)
	if !isExitHotkeySeq([]byte{0x1b, '[', '9', '9', ';', '6', 'u'}) {
		t.Fatalf("Ctrl+Alt+C CSI should exit with ctrl+alt config")
	}
	// CSI u Ctrl+Alt+Shift+C (mod=7, includes Ctrl+Alt)
	if !isExitHotkeySeq([]byte{0x1b, '[', '9', '9', ';', '7', 'u'}) {
		t.Fatalf("Ctrl+Alt+Shift+C should also exit")
	}
	// CSI u Ctrl+Shift+C (mod=5)
	if isExitHotkeySeq([]byte{0x1b, '[', '9', '9', ';', '5', 'u'}) {
		t.Fatalf("Ctrl+Shift+C should NOT exit with ctrl+alt config")
	}
	// CSI for other key
	if isExitHotkeySeq([]byte{0x1b, '[', '9', '7', ';', '6', 'u'}) {
		t.Fatalf("Ctrl+Alt+A should not exit")
	}

	// Simple ESC c (Alt+C) should NOT exit — requires Ctrl modifier
	if isExitHotkeySeq([]byte{0x1b, 'c'}) {
		t.Fatalf("Alt+C (ESC c) should NOT exit — Ctrl modifier required")
	}

	// Switch to ctrl+shift
	*cfg = Config{HotkeyMod: "ctrl+shift"}

	if !isExitHotkeySeq([]byte{0x1b, '[', '9', '9', ';', '5', 'u'}) {
		t.Fatalf("Ctrl+Shift+C should exit with ctrl+shift config")
	}
	if !isExitHotkeySeq([]byte{0x1b, '[', '9', '9', ';', '7', 'u'}) {
		t.Fatalf("Ctrl+Shift+Alt+C should also exit (includes Ctrl+Shift)")
	}
	if isExitHotkeySeq([]byte{0x1b, '[', '9', '9', ';', '6', 'u'}) {
		t.Fatalf("Ctrl+Alt+C should NOT exit with ctrl+shift config")
	}
	// Simple ESC c should NOT exit with ctrl+shift
	if isExitHotkeySeq([]byte{0x1b, 'c'}) {
		t.Fatalf("ESC c should NOT exit with ctrl+shift config")
	}
	// Non-CSI garbage
	if isExitHotkeySeq([]byte{0x1b, 'x'}) {
		t.Fatalf("ESC x should not exit")
	}
	if isExitHotkeySeq([]byte("hello")) {
		t.Fatalf("plain bytes should not exit")
	}

	*cfg = Config{HotkeyMod: "ctrl+alt"}
	// Ctrl only (mod=4) should not exit (requires Alt too)
	if isExitHotkeySeq([]byte{0x1b, '[', '9', '9', ';', '4', 'u'}) {
		t.Fatalf("Ctrl+C (without Alt) should not exit")
	}
	// Alt only (mod=2) should not exit (requires Ctrl too)
	if isExitHotkeySeq([]byte{0x1b, '[', '9', '9', ';', '2', 'u'}) {
		t.Fatalf("Alt+C (without Ctrl) should not exit")
	}
}
