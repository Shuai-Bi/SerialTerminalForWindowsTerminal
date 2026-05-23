package termapp

import (
	"path/filepath"
	"testing"

	appconfig "github.com/jixishi/SerialTerminalForWindowsTerminal/internal/config"
	"github.com/jixishi/SerialTerminalForWindowsTerminal/pkg/forward"
)

func TestForwardModeNetworkAndString(t *testing.T) {
	tests := []struct {
		mode    forward.Mode
		network string
		name    string
	}{
		{mode: forward.None, network: "", name: "none"},
		{mode: forward.TCP, network: "tcp", name: "tcp"},
		{mode: forward.UDP, network: "udp", name: "udp"},
	}

	for _, tt := range tests {
		if got := tt.mode.Network(); got != tt.network {
			t.Fatalf("Network() mode=%v got=%q want=%q", tt.mode, got, tt.network)
		}
		if got := tt.mode.String(); got != tt.name {
			t.Fatalf("String() mode=%v got=%q want=%q", tt.mode, got, tt.name)
		}
	}
}

func TestParseForwardMode(t *testing.T) {
	tests := []struct {
		input string
		mode  forward.Mode
		ok    bool
	}{
		{input: "tcp", mode: forward.TCP, ok: true},
		{input: "TCP-C", mode: forward.TCP, ok: true},
		{input: "1", mode: forward.TCP, ok: true},
		{input: "udp", mode: forward.UDP, ok: true},
		{input: " 2 ", mode: forward.UDP, ok: true},
		{input: "unknown", mode: forward.None, ok: false},
		{input: "", mode: forward.None, ok: false},
	}

	for _, tt := range tests {
		got, ok := forward.ParseMode(tt.input)
		if ok != tt.ok || got != tt.mode {
			t.Fatalf("forward.ParseMode(%q) got=(%v,%v) want=(%v,%v)", tt.input, got, ok, tt.mode, tt.ok)
		}
	}
}

func TestOpenLogFile(t *testing.T) {
	old := *cfg
	defer func() { *cfg = old }()

	*cfg = Config{
		EnableLog:   true,
		PortName:    "COM1",
		LogFilePath: filepath.Join(t.TempDir(), "%s-%s.log"),
	}

	f, err := appconfig.OpenLogFile(cfg)
	if err != nil {
		t.Fatalf("openLogFile() unexpected error: %v", err)
	}
	if f == nil {
		t.Fatalf("openLogFile() got nil file when enableLog=true")
	}
	_ = f.Close()

	cfg.EnableLog = false
	f, err = appconfig.OpenLogFile(cfg)
	if err != nil {
		t.Fatalf("openLogFile() unexpected error with enableLog=false: %v", err)
	}
	if f != nil {
		t.Fatalf("openLogFile() expected nil file when enableLog=false")
	}
}
