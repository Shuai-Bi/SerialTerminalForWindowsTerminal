package main

import (
	"fmt"
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
	"log"
	"os"
	"sort"
	"strconv"
	"strings"
)

type ptrVal struct {
	*string
	sl *[]string
	*int
	il *[]int
	*bool
	*float64
	*float32
	ext *string
}
type Val struct {
	string
	int
	bool
	float64
	float32
	extdef string
}
type Flag struct {
	v    ptrVal
	sStr string
	lStr string
	dv   Val
	help string
}

var (
	portName   = Flag{ptrVal{string: &cfg.PortName}, "p", "port", Val{string: ""}, "要连接的串口\t(/dev/ttyUSB0、COMx)"}
	baudRate   = Flag{ptrVal{int: &cfg.BaudRate}, "b", "baud", Val{int: 115200}, "波特率"}
	dataBits   = Flag{ptrVal{int: &cfg.DataBits}, "d", "data", Val{int: 8}, "数据位"}
	stopBits   = Flag{ptrVal{int: &cfg.StopBits}, "s", "stop", Val{int: 0}, "停止位停止位(0: 1停止 1:1.5停止 2:2停止)"}
	outputCode = Flag{ptrVal{string: &cfg.OutputCode}, "o", "out", Val{string: "UTF-8"}, "输出编码"}
	inputCode  = Flag{ptrVal{string: &cfg.InputCode}, "i", "in", Val{string: "UTF-8"}, "输入编码"}
	endStr     = Flag{ptrVal{string: &cfg.EndStr}, "e", "end", Val{string: "\n"}, "终端换行符"}
	logExt     = Flag{v: ptrVal{ext: &cfg.LogFilePath}, sStr: "l", lStr: "log", dv: Val{extdef: "./%s-$s.txt", string: ""}, help: "日志保存路径"}
	timeExt    = Flag{v: ptrVal{ext: &cfg.TimesFmt}, sStr: "t", lStr: "time", dv: Val{extdef: "[06-01-02 15:04:05.000]", string: ""}, help: "时间戳格式化字段"}
	forWard    = Flag{ptrVal{il: &cfg.ForWard}, "f", "forward", Val{int: 0}, "转发模式(0: 无 1:TCP-C 2:UDP-C 支持多次传入)"}
	address    = Flag{ptrVal{sl: &cfg.Address}, "a", "address", Val{string: "127.0.0.1:12345"}, "转发服务地址(支持多次传入)"}
	frameSize  = Flag{ptrVal{int: &cfg.FrameSize}, "F", "Frame", Val{int: 16}, "帧大小"}
	parityBit  = Flag{ptrVal{int: &cfg.ParityBit}, "v", "verify", Val{int: 0}, "奇偶校验(0:无校验、1:奇校验、2:偶校验、3:1校验、4:0校验)"}
	guiMode    = Flag{ptrVal{bool: &cfg.EnableGUI}, "g", "gui", Val{bool: false}, "启用TUI交互界面"}
	hotkeyMod  = Flag{ptrVal{string: &cfg.HotkeyMod}, "k", "hotkey-mod", Val{string: "ctrl+alt"}, "本地快捷键修饰(ctrl+alt|ctrl+shift)"}
	flags      = []Flag{portName, baudRate, dataBits, stopBits, outputCode, inputCode, endStr, forWard, address, frameSize, parityBit, logExt, timeExt, guiMode, hotkeyMod}
)

var (
	bauds = []string{"自定义", "300", "600", "1200", "2400", "4800", "9600",
		"14400", "19200", "38400", "56000", "57600", "115200", "128000",
		"256000", "460800", "512000", "750000", "921600", "1500000"}
	datas    = []string{"5", "6", "7", "8"}
	stops    = []string{"1", "1.5", "2"}
	paritys  = []string{"无校验", "奇校验", "偶校验", "1校验", "0校验"}
	forwards = []string{"No", "TCP-C", "UDP-C"}
)

type ValType int

const (
	notVal ValType = iota
	stringVal
	intVal
	boolVal
	extVal
	sliceStrVal
	sliceIntVal
)

func normalizeFlags() {
	known := make(map[string]bool, len(flags))
	for _, f := range flags {
		known[f.lStr] = true
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

func printUsage(ports []string) {
	sorted := make([]Flag, len(flags))
	copy(sorted, flags)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].lStr < sorted[j].lStr
	})

	fmt.Printf("\n参数帮助:\n")
	fmt.Printf("  %-6s %-14s %-8s %-44s %s\n", "短参", "长参", "类型", "说明", "默认值")
	fmt.Printf("  %-6s %-14s %-8s %-44s %s\n", "------", "------", "------", "------", "------")
	for _, f := range sorted {
		flagprint(f)
	}
	fmt.Printf("\n在线串口: %v\n", strings.Join(ports, ", "))
}

func flagFindValue(v ptrVal) ValType {
	if v.string != nil {
		return stringVal
	}
	if v.sl != nil {
		return sliceStrVal
	}
	if v.bool != nil {
		return boolVal
	}
	if v.int != nil {
		return intVal
	}
	if v.il != nil {
		return sliceIntVal
	}
	if v.ext != nil {
		return extVal
	}
	return notVal
}

func flagprint(f Flag) {
	short := "-" + f.sStr
	long := "--" + f.lStr
	help := f.help

	switch flagFindValue(f.v) {
	case stringVal:
		fmt.Printf("  %-6s %-14s %-8s %-44s %q\n", short, long, "string", help, f.dv.string)
	case intVal:
		fmt.Printf("  %-6s %-14s %-8s %-44s %v\n", short, long, "int", help, f.dv.int)
	case boolVal:
		fmt.Printf("  %-6s %-14s %-8s %-44s %v\n", short, long, "bool", help, f.dv.bool)
	case extVal:
		fmt.Printf("  %-6s %-14s %-8s %-44s %v\n", short, long, "string", help, f.dv.extdef)
	case sliceStrVal:
		fmt.Printf("  %-6s %-14s %-8s %-44s %q\n", short, long, "[]string", help, f.dv.string)
	case sliceIntVal:
		fmt.Printf("  %-6s %-14s %-8s %-44s %v\n", short, long, "[]int", help, f.dv.int)
	}
}
func flagInit(f *Flag) {
	if f.v.string != nil {
		pflag.StringVarP(f.v.string, f.lStr, f.sStr, f.dv.string, f.help)
	}
	if f.v.bool != nil {
		pflag.BoolVarP(f.v.bool, f.lStr, f.sStr, f.dv.bool, f.help)
	}
	if f.v.int != nil {
		pflag.IntVarP(f.v.int, f.lStr, f.sStr, f.dv.int, f.help)
	}
	if f.v.ext != nil {
		pflag.StringVarP(f.v.ext, f.lStr, f.sStr, f.dv.string, f.help)
		pflag.Lookup(f.lStr).NoOptDefVal = f.dv.extdef
	}
	if f.v.sl != nil {
		pflag.StringArrayVarP(f.v.sl, f.lStr, f.sStr, []string{f.dv.string}, f.help)
	}
	if f.v.il != nil {
		pflag.IntSliceVarP(f.v.il, f.lStr, f.sStr, []int{f.dv.int}, f.help)
	}
}
func flagExt() {
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
func getCliFlag() {
	ports, err := serial.GetPortsList()
	if err != nil {
		log.Fatal(err)
	}

	inputs := components.NewInput()
	inputs.Prompt = "Filtering: "
	inputs.PromptStyle = style.New().Bold().Italic().Fg(color.LightBlue)

	selectKeymap := singleselect.DefaultSingleKeyMap()
	selectKeymap.Confirm = key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "finish select"),
	)
	selectKeymap.Choice = key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "finish select"),
	)
	selectKeymap.NextPage = key.NewBinding(
		key.WithKeys("right"),
		key.WithHelp("->", "next page"),
	)
	selectKeymap.PrevPage = key.NewBinding(
		key.WithKeys("left"),
		key.WithHelp("<-", "prev page"),
	)

	s, _ := inf.NewSingleSelect(
		ports,
		singleselect.WithKeyBinding(selectKeymap),
		singleselect.WithPageSize(4),
		singleselect.WithFilterInput(inputs),
	).Display("选择串口")
	cfg.PortName = ports[s]

	s, _ = inf.NewSingleSelect(
		bauds,
		singleselect.WithKeyBinding(selectKeymap),
		singleselect.WithPageSize(4),
	).Display("选择波特率")
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
	v, _ := inf.NewConfirmWithSelection(
		confirm.WithPrompt("启用Hex"),
	).Display()
	if v {
		cfg.InputCode = "hex"
		b, _ := inf.NewText(
			text.WithPrompt("Frames:"),
			text.WithPromptStyle(theme.DefaultTheme.PromptStyle),
			text.WithDefaultValue("16"),
		).Display()
		cfg.FrameSize, _ = strconv.Atoi(b)
	}
	v, _ = inf.NewConfirmWithSelection(
		confirm.WithPrompt("启用时间戳"),
	).Display()
	cfg.TimesTamp = v
	if v {
		b, _ := inf.NewText(
			text.WithPrompt("格式化字段:"),
			text.WithPromptStyle(theme.DefaultTheme.PromptStyle),
			text.WithDefaultValue(timeExt.dv.extdef),
		).Display()
		cfg.TimesFmt = b
	}
	v, _ = inf.NewConfirmWithSelection(
		confirm.WithPrompt("启用高级配置"),
	).Display()
	if v {
		s, _ = inf.NewSingleSelect(
			datas,
			singleselect.WithKeyBinding(selectKeymap),
			singleselect.WithPageSize(4),
			singleselect.WithFilterInput(inputs),
		).Display("选择数据位")
		cfg.DataBits, _ = strconv.Atoi(datas[s])

		s, _ = inf.NewSingleSelect(
			stops,
			singleselect.WithKeyBinding(selectKeymap),
			singleselect.WithPageSize(4),
			singleselect.WithFilterInput(inputs),
		).Display("选择停止位")
		cfg.StopBits = s

		s, _ = inf.NewSingleSelect(
			paritys,
			singleselect.WithKeyBinding(selectKeymap),
			singleselect.WithPageSize(4),
			singleselect.WithFilterInput(inputs),
		).Display("选择校验位")
		cfg.ParityBit = s

		t, _ := inf.NewText(
			text.WithPrompt("换行符:"),
			text.WithPromptStyle(theme.DefaultTheme.PromptStyle),
			text.WithDefaultValue(endStr.dv.string),
		).Display()
		cfg.EndStr = t

		v, _ = inf.NewConfirmWithSelection(
			confirm.WithDefaultYes(),
			confirm.WithPrompt("启用编码转换"),
		).Display()

		if v {
			t, _ = inf.NewText(
				text.WithPrompt("输入编码:"),
				text.WithPromptStyle(theme.DefaultTheme.PromptStyle),
				text.WithDefaultValue(inputCode.dv.string),
			).Display()
			cfg.InputCode = t

			t, _ = inf.NewText(
				text.WithPrompt("输出编码:"),
				text.WithPromptStyle(theme.DefaultTheme.PromptStyle),
				text.WithDefaultValue(outputCode.dv.string),
			).Display()
			cfg.OutputCode = t
		}
	G_F_mode:
		s, _ = inf.NewSingleSelect(
			forwards,
			singleselect.WithKeyBinding(selectKeymap),
			singleselect.WithPageSize(3),
			singleselect.WithFilterInput(inputs),
		).Display("选择转发模式")
		if s != 0 {
			cfg.ForWard = append(cfg.ForWard, s)
			t, _ = inf.NewText(
				text.WithPrompt("地址:"),
				text.WithPromptStyle(theme.DefaultTheme.PromptStyle),
				text.WithDefaultValue(address.dv.string),
			).Display()
			cfg.Address = append(cfg.Address, t)
			goto G_F_mode
		}

		e, _ := inf.NewConfirmWithSelection(
			confirm.WithDefaultYes(),
			confirm.WithPrompt("启用日志"),
		).Display()
		cfg.EnableLog = e
		if e {
			t, _ = inf.NewText(
				text.WithPrompt("Path:"),
				text.WithPromptStyle(theme.DefaultTheme.PromptStyle),
				text.WithDefaultValue("./%s-$s.txt"),
			).Display()
			cfg.LogFilePath = t
		}
	}

}
