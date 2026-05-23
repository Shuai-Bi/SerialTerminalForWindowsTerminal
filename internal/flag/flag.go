// Package flag provides CLI flag parsing and interactive configuration.
package flag

import (
	"fmt"
	"log"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	inf "github.com/fzdwx/infinite"
	"github.com/fzdwx/infinite/color"
	"github.com/fzdwx/infinite/components"
	"github.com/fzdwx/infinite/components/input/text"
	"github.com/fzdwx/infinite/components/selection/confirm"
	"github.com/fzdwx/infinite/components/selection/singleselect"
	"github.com/fzdwx/infinite/style"
	"github.com/fzdwx/infinite/theme"
	"github.com/spf13/pflag"
	"go.bug.st/serial"

	"github.com/jixishi/SerialTerminalForWindowsTerminal/internal/config"
)

// Init registers all CLI flags with pflag, binding them to the given config.
func Init(cfg *config.Config) {
	pflag.StringVarP(&cfg.PortName, "port", "p", "", "serial port (/dev/ttyUSB0, COMx)")
	pflag.IntVarP(&cfg.BaudRate, "baud", "b", 115200, "baud rate")
	pflag.IntVarP(&cfg.DataBits, "data", "d", 8, "data bits")
	pflag.IntVarP(&cfg.StopBits, "stop", "s", 0, "stop bits (0:1, 1:1.5, 2:2)")
	pflag.StringVarP(&cfg.OutputCode, "out", "o", "UTF-8", "output charset")
	pflag.StringVarP(&cfg.InputCode, "in", "i", "UTF-8", "input charset")
	pflag.StringVarP(&cfg.EndStr, "end", "e", "\n", "line ending")
	pflag.IntVarP(&cfg.FrameSize, "Frame", "F", 16, "hex frame size")
	pflag.IntVarP(&cfg.ParityBit, "verify", "v", 0, "parity (0:none,1:odd,2:even,3:mark,4:space)")
	pflag.BoolVarP(&cfg.EnableGUI, "gui", "g", false, "enable TUI mode")
	pflag.StringVarP(&cfg.HotkeyMod, "hotkey-mod", "k", "ctrl+alt", "hotkey modifier (ctrl+alt|ctrl+shift)")
	pflag.IntSliceVarP(&cfg.ForWard, "forward", "f", nil, "forward mode (0:none,1:TCP,2:UDP,3:TCP-S,4:UDP-S,5:COM)")
	pflag.StringArrayVarP(&cfg.Address, "address", "a", nil, "forward address")
	pflag.StringVarP(&cfg.LogFilePath, "log", "l", "", "log file path")
	_ = pflag.Lookup("log") // mark for NoOptDefVal
	pflag.StringVarP(&cfg.TimesFmt, "time", "t", "", "timestamp format")
	_ = pflag.Lookup("time") // mark for NoOptDefVal
}

// Normalize converts single-dash long flags (e.g. -port) to double-dash (--port).
// Parse wraps pflag.Parse.
func Parse() { pflag.Parse() }

// Normalize converts single-dash long flags (e.g. -port) to double-dash (--port).
func Normalize() {
	known := map[string]bool{
		"port": true, "baud": true, "data": true, "stop": true,
		"out": true, "in": true, "end": true, "Frame": true,
		"verify": true, "gui": true, "hotkey-mod": true,
		"forward": true, "address": true, "log": true, "time": true,
	}
	for i, arg := range os.Args[1:] {
		if strings.HasPrefix(arg, "-") && !strings.HasPrefix(arg, "--") {
			name := strings.TrimPrefix(arg, "-")
			if known[name] {
				os.Args[i+1] = "--" + name
			}
		}
	}
}

// Ext applies post-parse normalization to config values.
func Ext(cfg *config.Config) {
	if cfg.LogFilePath != "" {
		cfg.EnableLog = true
	}
	if cfg.TimesFmt != "" {
		cfg.TimesTamp = true
	}
	if cfg.HotkeyMod == "" {
		cfg.HotkeyMod = "ctrl+alt"
	}
	cfg.HotkeyMod = strings.ToLower(strings.TrimSpace(cfg.HotkeyMod))
	if cfg.HotkeyMod != "ctrl+alt" && cfg.HotkeyMod != "ctrl+shift" {
		cfg.HotkeyMod = "ctrl+alt"
	}
}

// PrintUsage displays flag help and available ports.
func PrintUsage(ports []string) {
	type flagInfo struct{ short, long, typ, help, def string }
	flags := []flagInfo{
		{"-p", "--port", "string", "serial port", ""},
		{"-b", "--baud", "int", "baud rate", "115200"},
		{"-d", "--data", "int", "data bits", "8"},
		{"-s", "--stop", "int", "stop bits", "0"},
		{"-o", "--out", "string", "output charset", "UTF-8"},
		{"-i", "--in", "string", "input charset", "UTF-8"},
		{"-e", "--end", "string", "line ending", "\\n"},
		{"-F", "--Frame", "int", "hex frame size", "16"},
		{"-v", "--verify", "int", "parity", "0"},
		{"-g", "--gui", "bool", "enable TUI", "false"},
		{"-k", "--hotkey-mod", "string", "hotkey modifier", "ctrl+alt"},
		{"-f", "--forward", "[]int", "forward (0:none,1:TCP,2:UDP,3:TCP-S,4:UDP-S,5:COM)", "0"},
		{"-a", "--address", "[]string", "forward address", "127.0.0.1:12345"},
		{"-l", "--log", "string", "log path (%s=port, then timestamp)", "./%s-%s.log"},
		{"-t", "--time", "string", "timestamp format", "[06-01-02 15:04:05.000]"},
	}
	sort.Slice(flags, func(i, j int) bool { return flags[i].long < flags[j].long })

	fmt.Printf("\nFlags:\n")
	fmt.Printf("  %-6s %-14s %-8s %-44s %s\n", "Short", "Long", "Type", "Help", "Default")
	fmt.Printf("  %-6s %-14s %-8s %-44s %s\n", "------", "------", "------", "------", "------")
	for _, f := range flags {
		fmt.Printf("  %-6s %-14s %-8s %-44s %q\n", f.short, f.long, f.typ, f.help, f.def)
	}
	fmt.Printf("\nAvailable ports: %v\n", strings.Join(ports, ", "))
}

var (
	bauds = []string{"Custom", "300", "600", "1200", "2400", "4800", "9600",
		"14400", "19200", "38400", "56000", "57600", "115200", "128000",
		"256000", "460800", "512000", "750000", "921600", "1500000"}
	datas    = []string{"5", "6", "7", "8"}
	stops    = []string{"1", "1.5", "2"}
	paritys  = []string{"None", "Odd", "Even", "Mark", "Space"}
	forwards = []string{"No", "TCP-C", "UDP-C", "TCP-S", "UDP-S", "COM"}
)

// GetCliFlag runs an interactive configuration wizard when no port is specified.
func GetCliFlag(cfg *config.Config) {
	ports, err := serial.GetPortsList()
	if err != nil {
		log.Fatal(err)
	}

	inputs := components.NewInput()
	inputs.Prompt = "Filtering: "
	inputs.PromptStyle = style.New().Bold().Italic().Fg(color.LightBlue)

	selectKeymap := singleselect.DefaultSingleKeyMap()
	selectKeymap.Confirm = key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "finish select"))
	selectKeymap.Choice = key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "finish select"))
	selectKeymap.NextPage = key.NewBinding(key.WithKeys("right"), key.WithHelp("->", "next page"))
	selectKeymap.PrevPage = key.NewBinding(key.WithKeys("left"), key.WithHelp("<-", "prev page"))

	s, _ := inf.NewSingleSelect(ports,
		singleselect.WithKeyBinding(selectKeymap),
		singleselect.WithPageSize(4),
		singleselect.WithFilterInput(inputs),
	).Display("Select serial port")
	cfg.PortName = ports[s]

	s, _ = inf.NewSingleSelect(bauds,
		singleselect.WithKeyBinding(selectKeymap),
		singleselect.WithPageSize(4),
	).Display("Select baud rate")
	if s != 0 {
		cfg.BaudRate, _ = strconv.Atoi(bauds[s])
	} else {
		b, _ := inf.NewText(
			text.WithPrompt("BaudRate:"),
			text.WithPromptStyle(theme.DefaultTheme.PromptStyle),
			text.WithDefaultValue("115200"),
		).Display()
		cfg.BaudRate, _ = strconv.Atoi(b)
	}

	v, _ := inf.NewConfirmWithSelection(confirm.WithPrompt("Enable Hex")).Display()
	if v {
		cfg.InputCode = "hex"
		b, _ := inf.NewText(
			text.WithPrompt("Frames:"),
			text.WithPromptStyle(theme.DefaultTheme.PromptStyle),
			text.WithDefaultValue("16"),
		).Display()
		cfg.FrameSize, _ = strconv.Atoi(b)
	}

	v, _ = inf.NewConfirmWithSelection(confirm.WithPrompt("Enable Timestamp")).Display()
	cfg.TimesTamp = v
	if v {
		b, _ := inf.NewText(
			text.WithPrompt("Format:"),
			text.WithPromptStyle(theme.DefaultTheme.PromptStyle),
			text.WithDefaultValue("[06-01-02 15:04:05.000]"),
		).Display()
		cfg.TimesFmt = b
	}

	v, _ = inf.NewConfirmWithSelection(confirm.WithPrompt("Enable advanced config")).Display()
	if v {
		s, _ = inf.NewSingleSelect(datas,
			singleselect.WithKeyBinding(selectKeymap),
			singleselect.WithPageSize(4),
			singleselect.WithFilterInput(inputs),
		).Display("Select data bits")
		cfg.DataBits, _ = strconv.Atoi(datas[s])

		s, _ = inf.NewSingleSelect(stops,
			singleselect.WithKeyBinding(selectKeymap),
			singleselect.WithPageSize(4),
			singleselect.WithFilterInput(inputs),
		).Display("Select stop bits")
		cfg.StopBits = s

		s, _ = inf.NewSingleSelect(paritys,
			singleselect.WithKeyBinding(selectKeymap),
			singleselect.WithPageSize(4),
			singleselect.WithFilterInput(inputs),
		).Display("Select parity")
		cfg.ParityBit = s

		t, _ := inf.NewText(
			text.WithPrompt("Line ending:"),
			text.WithPromptStyle(theme.DefaultTheme.PromptStyle),
			text.WithDefaultValue("\n"),
		).Display()
		cfg.EndStr = t

		v, _ = inf.NewConfirmWithSelection(confirm.WithDefaultYes(), confirm.WithPrompt("Enable charset conversion")).Display()
		if v {
			t, _ = inf.NewText(
				text.WithPrompt("Input charset:"),
				text.WithPromptStyle(theme.DefaultTheme.PromptStyle),
				text.WithDefaultValue("UTF-8"),
			).Display()
			cfg.InputCode = t

			t, _ = inf.NewText(
				text.WithPrompt("Output charset:"),
				text.WithPromptStyle(theme.DefaultTheme.PromptStyle),
				text.WithDefaultValue("UTF-8"),
			).Display()
			cfg.OutputCode = t
		}

	G_F_mode:
		s, _ = inf.NewSingleSelect(forwards,
			singleselect.WithKeyBinding(selectKeymap),
			singleselect.WithPageSize(3),
			singleselect.WithFilterInput(inputs),
		).Display("Select forward mode")
		if s != 0 {
			cfg.ForWard = append(cfg.ForWard, s)
			t, _ = inf.NewText(
				text.WithPrompt("Address:"),
				text.WithPromptStyle(theme.DefaultTheme.PromptStyle),
				text.WithDefaultValue("127.0.0.1:12345"),
			).Display()
			cfg.Address = append(cfg.Address, t)
			goto G_F_mode
		}

		e, _ := inf.NewConfirmWithSelection(confirm.WithDefaultYes(), confirm.WithPrompt("Enable logging")).Display()
		cfg.EnableLog = e
		if e {
			t, _ = inf.NewText(
				text.WithPrompt("Path(%s=port, then stamp):"),
				text.WithPromptStyle(theme.DefaultTheme.PromptStyle),
				text.WithDefaultValue("./%s-%s.log"),
			).Display()
			cfg.LogFilePath = t
		}
	}
}
