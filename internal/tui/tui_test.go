package tui

import "testing"

func TestParseCSIuBytes(t *testing.T) {
	tests := []struct {
		name string
		seq  []byte
		want string
		ok   bool
	}{
		{name: "ctrl+alt+f", seq: []byte{0x1b, '[', '1', '0', '2', ';', '6', 'u'}, want: "ctrl+alt+f", ok: true},
		{name: "ctrl+alt+c", seq: []byte{0x1b, '[', '9', '9', ';', '6', 'u'}, want: "ctrl+alt+c", ok: true},
		{name: "ctrl+alt+m", seq: []byte{0x1b, '[', '1', '0', '9', ';', '6', 'u'}, want: "ctrl+alt+m", ok: true},
		{name: "ctrl+alt+p", seq: []byte{0x1b, '[', '1', '1', '2', ';', '6', 'u'}, want: "ctrl+alt+p", ok: true},
		{name: "ctrl+alt+h", seq: []byte{0x1b, '[', '1', '0', '4', ';', '6', 'u'}, want: "ctrl+alt+h", ok: true},
		{name: "ctrl+shift+c", seq: []byte{0x1b, '[', '9', '9', ';', '5', 'u'}, want: "ctrl+shift+c", ok: true},
		{name: "alt+c (no ctrl)", seq: []byte{0x1b, '[', '9', '9', ';', '2', 'u'}, want: "alt+c", ok: true},
		{name: "plain c", seq: []byte{0x1b, '[', '9', '9', ';', '0', 'u'}, want: "c", ok: true},
		{name: "not CSI u", seq: []byte{0x1b, '[', 'A'}, want: "", ok: false},
		{name: "empty", seq: []byte{}, want: "", ok: false},
		{name: "no escape", seq: []byte("hello"), want: "", ok: false},
		{name: "ESC [ A (arrow up)", seq: []byte{0x1b, '[', 'A'}, want: "", ok: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := parseCSIuBytes(tt.seq)
			if ok != tt.ok || got != tt.want {
				t.Fatalf("parseCSIuBytes(%v): got=(%q,%v) want=(%q,%v)", tt.seq, got, ok, tt.want, tt.ok)
			}
		})
	}
}
