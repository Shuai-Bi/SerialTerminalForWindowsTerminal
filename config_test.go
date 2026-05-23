package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/pflag"

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
	old := config
	defer func() { config = old }()

	config = Config{
		enableLog:   true,
		portName:    "COM1",
		logFilePath: filepath.Join(t.TempDir(), "%s-%s.log"),
	}

	f, err := openLogFile()
	if err != nil {
		t.Fatalf("openLogFile() unexpected error: %v", err)
	}
	if f == nil {
		t.Fatalf("openLogFile() got nil file when enableLog=true")
	}
	_ = f.Close()

	config.enableLog = false
	f, err = openLogFile()
	if err != nil {
		t.Fatalf("openLogFile() unexpected error with enableLog=false: %v", err)
	}
	if f != nil {
		t.Fatalf("openLogFile() expected nil file when enableLog=false")
	}
}

func TestFlagFindValue(t *testing.T) {
	s := "str"
	sl := []string{"a"}
	n := 1
	il := []int{1}
	b := true
	ext := "ext"

	tests := []struct {
		name string
		v    ptrVal
		want ValType
	}{
		{name: "string", v: ptrVal{string: &s}, want: stringVal},
		{name: "stringSlice", v: ptrVal{sl: &sl}, want: sliceStrVal},
		{name: "bool", v: ptrVal{bool: &b}, want: boolVal},
		{name: "int", v: ptrVal{int: &n}, want: intVal},
		{name: "intSlice", v: ptrVal{il: &il}, want: sliceIntVal},
		{name: "ext", v: ptrVal{ext: &ext}, want: extVal},
		{name: "none", v: ptrVal{}, want: notVal},
	}

	for _, tt := range tests {
		got := flagFindValue(tt.v)
		if got != tt.want {
			t.Fatalf("%s: flagFindValue got=%v want=%v", tt.name, got, tt.want)
		}
	}
}

func TestFlagExt(t *testing.T) {
	old := config
	defer func() { config = old }()

	config = Config{}
	flagExt()
	if config.enableLog {
		t.Fatalf("expected enableLog=false when logFilePath empty")
	}
	if config.timesTamp {
		t.Fatalf("expected timesTamp=false when timesFmt empty")
	}
	if config.hotkeyMod != "ctrl+alt" {
		t.Fatalf("expected default hotkeyMod=ctrl+alt, got=%q", config.hotkeyMod)
	}

	config = Config{logFilePath: "/tmp/log.txt"}
	flagExt()
	if !config.enableLog {
		t.Fatalf("expected enableLog=true when logFilePath set")
	}

	config = Config{timesFmt: "2006-01-02"}
	flagExt()
	if !config.timesTamp {
		t.Fatalf("expected timesTamp=true when timesFmt set")
	}

	config = Config{hotkeyMod: ""}
	flagExt()
	if config.hotkeyMod != "ctrl+alt" {
		t.Fatalf("empty hotkeyMod should default to ctrl+alt")
	}

	config = Config{hotkeyMod: "ctrl+shift"}
	flagExt()
	if config.hotkeyMod != "ctrl+shift" {
		t.Fatalf("expected ctrl+shift preserved")
	}

	config = Config{hotkeyMod: "  CTRL+SHIFT  "}
	flagExt()
	if config.hotkeyMod != "ctrl+shift" {
		t.Fatalf("expected whitespace+case normalization, got=%q", config.hotkeyMod)
	}

	config = Config{hotkeyMod: "invalid"}
	flagExt()
	if config.hotkeyMod != "ctrl+alt" {
		t.Fatalf("invalid hotkeyMod should default to ctrl+alt, got=%q", config.hotkeyMod)
	}
}

func TestFlagInit(t *testing.T) {
	var testStr string
	var testBool bool
	var testInt int
	var testExt string
	var testSl []string
	var testIl []int

	f := Flag{
		v:    ptrVal{string: &testStr},
		sStr: "X", lStr: "test-str", dv: Val{string: "hello"}, help: "test string",
	}
	flagInit(&f)
	if pflag.Lookup("test-str") == nil {
		t.Fatalf("string flag not registered")
	}

	boolF := Flag{
		v:    ptrVal{bool: &testBool},
		sStr: "Y", lStr: "test-bool", dv: Val{bool: true}, help: "test bool",
	}
	flagInit(&boolF)

	intF := Flag{
		v:    ptrVal{int: &testInt},
		sStr: "Z", lStr: "test-int", dv: Val{int: 42}, help: "test int",
	}
	flagInit(&intF)

	extF := Flag{
		v:    ptrVal{ext: &testExt},
		sStr: "E", lStr: "test-ext", dv: Val{extdef: "default-val", string: ""}, help: "test ext",
	}
	flagInit(&extF)

	slF := Flag{
		v:    ptrVal{sl: &testSl},
		sStr: "1", lStr: "test-sl", dv: Val{string: "a"}, help: "test sl",
	}
	flagInit(&slF)

	ilF := Flag{
		v:    ptrVal{il: &testIl},
		sStr: "2", lStr: "test-il", dv: Val{int: 1}, help: "test il",
	}
	flagInit(&ilF)
}

func TestNormalizeFlags(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	os.Args = []string{"COM.exe", "-port", "COM17", "-baud", "9600", "-p", "COM1", "--gui", "COM17"}
	normalizeFlags()

	args := os.Args
	if args[1] != "--port" {
		t.Fatalf("expected -port -> --port, got %q", args[1])
	}
	if args[3] != "--baud" {
		t.Fatalf("expected -baud -> --baud, got %q", args[3])
	}
	if args[5] != "-p" {
		t.Fatalf("expected -p unchanged, got %q", args[5])
	}
	if args[7] != "--gui" {
		t.Fatalf("expected --gui unchanged, got %q", args[7])
	}
	if args[8] != "COM17" {
		t.Fatalf("expected value unchanged, got %q", args[8])
	}
}
