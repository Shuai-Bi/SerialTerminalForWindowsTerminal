package main

import (
	"encoding/hex"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/jixishi/SerialTerminalForWindowsTerminal/internal/event"
	"github.com/jixishi/SerialTerminalForWindowsTerminal/pkg/forward"
)

type CommandHandler func(args []string) error
type CommandCompleter func(args []string) []string

type RuntimeCommand struct {
	Name        string
	Usage       string
	Description string
	Handler     CommandHandler
	Completer   CommandCompleter
}

type CommandDispatcher struct {
	app      *App
	commands map[string]*RuntimeCommand
	order    []string
}

func NewCommandDispatcher(app *App) *CommandDispatcher {
	d := &CommandDispatcher{
		app:      app,
		commands: make(map[string]*RuntimeCommand),
	}

	d.registerAll()
	return d
}

func (d *CommandDispatcher) register(cmd RuntimeCommand) {
	key := strings.ToLower(cmd.Name)
	d.commands[key] = &cmd
	d.order = append(d.order, key)
}

func (d *CommandDispatcher) registerAll() {
	d.register(RuntimeCommand{
		Name:        ".help",
		Usage:       ".help",
		Description: "show command help",
		Handler: func(args []string) error {
			d.app.ShowModal("Command Help", d.HelpText())
			return nil
		},
	})

	d.register(RuntimeCommand{
		Name:        ".exit",
		Usage:       ".exit",
		Description: "exit local terminal",
		Handler: func(args []string) error {
			d.app.Statusf("[local] exiting")
			d.app.Close()
			return nil
		},
	})

	d.register(RuntimeCommand{
		Name:        ".hex",
		Usage:       ".hex <hex-data>",
		Description: "send raw hex bytes",
		Handler: func(args []string) error {
			if len(args) < 2 {
				return fmt.Errorf("usage: .hex <hex-data>")
			}
			hexStr := strings.Join(args[1:], "")
			b, err := hex.DecodeString(hexStr)
			if err != nil {
				return err
			}
			return d.app.writeToSession(b)
		},
	})

	d.register(RuntimeCommand{
		Name:        ".forward",
		Usage:       ".forward <list|add|remove|enable|disable|update|stats>",
		Description: "manage forwarding at runtime",
		Handler:     d.handleForwardCommand,
		Completer:   completeForward,
	})

	d.register(RuntimeCommand{
		Name:        ".plugin",
		Usage:       ".plugin <list|load|unload|enable|disable|reload>",
		Description: "manage lua plugins",
		Handler:     d.handlePluginCommand,
		Completer:   completePlugin,
	})

	d.register(RuntimeCommand{
		Name:        ".mode",
		Usage:       ".mode <show|set>",
		Description: "show or update runtime terminal mode",
		Handler: func(args []string) error {
			return d.handleModeCommand(args)
		},
		Completer: completeMode,
	})
}

func (d *CommandDispatcher) Execute(line string) (bool, error) {
	args := strings.Fields(strings.TrimSpace(line))
	if len(args) == 0 {
		return false, nil
	}
	if !strings.HasPrefix(args[0], ".") {
		return false, nil
	}

	cmd, ok := d.commands[strings.ToLower(args[0])]
	if !ok {
		return true, fmt.Errorf("unknown command: %s", args[0])
	}

	if err := cmd.Handler(args); err != nil {
		return true, err
	}
	return true, nil
}

func (d *CommandDispatcher) HelpText() string {
	keys := make([]string, 0, len(d.order))
	for _, k := range d.order {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var b strings.Builder
	b.WriteString("Commands:\n")
	for _, k := range keys {
		cmd := d.commands[k]
		b.WriteString(fmt.Sprintf("  %-12s %-40s %s\n", cmd.Name, cmd.Usage, cmd.Description))
	}
	return b.String()
}

func (d *CommandDispatcher) Complete(line string) (string, []string) {
	trimmed := strings.TrimLeft(line, " ")
	if trimmed == "" {
		return line, nil
	}

	args := strings.Fields(trimmed)
	endsWithSpace := strings.HasSuffix(line, " ")

	if len(args) == 0 {
		return line, nil
	}

	if len(args) == 1 && !endsWithSpace {
		return completeFirstToken(line, args[0], d.commandNames())
	}

	cmdName := strings.ToLower(args[0])
	cmd, ok := d.commands[cmdName]
	if !ok || cmd.Completer == nil {
		return line, nil
	}

	compArgs := args
	if endsWithSpace {
		compArgs = append(compArgs, "")
	}

	cands := cmd.Completer(compArgs)
	if len(cands) == 0 {
		return line, nil
	}

	current := compArgs[len(compArgs)-1]
	base := strings.TrimSuffix(line, current)

	matches := filterPrefix(cands, current)
	if len(matches) == 0 {
		matches = cands
	}
	if len(matches) == 1 {
		return base + matches[0], matches
	}

	return line, matches
}

func (d *CommandDispatcher) commandNames() []string {
	names := make([]string, 0, len(d.commands))
	for _, cmd := range d.commands {
		names = append(names, cmd.Name)
	}
	sort.Strings(names)
	return names
}

func completeFirstToken(line, token string, cands []string) (string, []string) {
	matches := filterPrefix(cands, token)
	if len(matches) == 0 {
		return line, nil
	}
	if len(matches) == 1 {
		prefix := strings.TrimSuffix(line, token)
		return prefix + matches[0] + " ", matches
	}
	return line, matches
}

func filterPrefix(cands []string, cur string) []string {
	if cur == "" {
		return append([]string{}, cands...)
	}
	res := make([]string, 0, len(cands))
	for _, c := range cands {
		if strings.HasPrefix(strings.ToLower(c), strings.ToLower(cur)) {
			res = append(res, c)
		}
	}
	return res
}

func completeForward(args []string) []string {
	if len(args) <= 2 {
		return []string{"list", "add", "remove", "enable", "disable", "update", "stats"}
	}

	if len(args) == 3 && args[1] == "add" {
		return []string{"tcp", "udp"}
	}

	if len(args) == 4 && args[1] == "update" {
		return []string{"tcp", "udp"}
	}

	return nil
}

func completePlugin(args []string) []string {
	if len(args) <= 2 {
		return []string{"list", "load", "unload", "enable", "disable", "reload"}
	}
	return nil
}

func completeMode(args []string) []string {
	if len(args) <= 2 {
		return []string{"show", "set"}
	}

	if len(args) == 3 && args[1] == "set" {
		return []string{"in", "out", "end", "frame", "timestamp", "timefmt"}
	}

	if len(args) == 4 && args[1] == "set" && args[2] == "timestamp" {
		return []string{"on", "off"}
	}

	return nil
}

func (d *CommandDispatcher) handleForwardCommand(args []string) error {
	if len(args) < 2 {
		if d.app.UIEnabled() {
			d.app.OpenPanel(event.UIPanelForward)
			return nil
		}
		args = []string{".forward", "list"}
	}

	sub := strings.ToLower(args[1])
	switch sub {
	case "list", "stats":
		if d.app.UIEnabled() {
			d.app.OpenPanel(event.UIPanelForward)
			return nil
		}

		items := d.app.forward.List()
		if len(items) == 0 {
			d.app.Notifyf("[forward] empty")
			return nil
		}
		d.app.Notifyf("[forward] ID Mode Enabled Connected Address InBytes OutBytes LastError")
		for _, it := range items {
			d.app.Notifyf("[forward] %d %s %v %v %s %d %d %s", it.ID, it.Mode, it.Enabled, it.Connected, it.Address, it.ReadBytes, it.WriteByte, it.LastError)
		}
		return nil

	case "add":
		if len(args) < 4 {
			return fmt.Errorf("usage: .forward add <tcp|udp> <address>")
		}
		mode, ok := forward.ParseMode(args[2])
		if !ok {
			return fmt.Errorf("unknown forward mode: %s", args[2])
		}
		id, err := d.app.forward.Add(mode, args[3])
		if err != nil {
			return err
		}
		d.app.Statusf("[forward] added #%d", id)
		return nil

	case "remove", "enable", "disable":
		if len(args) < 3 {
			return fmt.Errorf("usage: .forward %s <id>", sub)
		}
		id, err := strconv.Atoi(args[2])
		if err != nil {
			return err
		}
		switch sub {
		case "remove":
			return d.app.forward.Remove(id)
		case "enable":
			return d.app.forward.Enable(id)
		case "disable":
			return d.app.forward.Disable(id)
		}

	case "update":
		if len(args) < 5 {
			return fmt.Errorf("usage: .forward update <id> <tcp|udp> <address>")
		}
		id, err := strconv.Atoi(args[2])
		if err != nil {
			return err
		}
		mode, ok := forward.ParseMode(args[3])
		if !ok {
			return fmt.Errorf("unknown forward mode: %s", args[3])
		}
		if err = d.app.forward.Update(id, mode, args[4]); err != nil {
			return err
		}
		d.app.Statusf("[forward] updated #%d", id)
		return nil
	}

	return fmt.Errorf("unknown subcommand: %s", sub)
}

func (d *CommandDispatcher) handlePluginCommand(args []string) error {
	if len(args) < 2 {
		if d.app.UIEnabled() {
			d.app.OpenPanel(event.UIPanelPlugin)
			return nil
		}
		args = []string{".plugin", "list"}
	}

	sub := strings.ToLower(args[1])
	switch sub {
	case "list":
		if d.app.UIEnabled() {
			d.app.OpenPanel(event.UIPanelPlugin)
			return nil
		}

		items := d.app.plugins.List()
		if len(items) == 0 {
			d.app.Notifyf("[plugin] empty")
			return nil
		}
		for _, it := range items {
			d.app.Notifyf("[plugin] %s enabled=%v path=%s", it.Name, it.Enabled, it.Path)
		}
		return nil

	case "load":
		if len(args) < 3 {
			return fmt.Errorf("usage: .plugin load <path>")
		}
		name, err := d.app.plugins.Load(args[2])
		if err != nil {
			return err
		}
		d.app.Statusf("[plugin] loaded %s", name)
		return nil

	case "unload", "enable", "disable", "reload":
		if len(args) < 3 {
			return fmt.Errorf("usage: .plugin %s <name>", sub)
		}
		name := args[2]
		switch sub {
		case "unload":
			return d.app.plugins.Unload(name)
		case "enable":
			return d.app.plugins.Enable(name)
		case "disable":
			return d.app.plugins.Disable(name)
		case "reload":
			return d.app.plugins.Reload(name)
		}
	}

	return fmt.Errorf("unknown subcommand: %s", sub)
}

func (d *CommandDispatcher) handleModeCommand(args []string) error {
	if len(args) < 2 || strings.EqualFold(args[1], "show") {
		if d.app.UIEnabled() {
			d.app.OpenPanel(event.UIPanelMode)
			return nil
		}

		d.app.Notifyf("[mode] input=%s output=%s end=%q hex=%v frame=%d timestamp=%v timefmt=%q forwardTargets=%d plugins=%d",
			d.app.cfg.inputCode,
			d.app.cfg.outputCode,
			d.app.cfg.endStr,
			strings.EqualFold(d.app.cfg.inputCode, "hex"),
			d.app.cfg.frameSize,
			d.app.cfg.timesTamp,
			d.app.cfg.timesFmt,
			len(d.app.forward.List()),
			len(d.app.plugins.List()),
		)
		return nil
	}

	if !strings.EqualFold(args[1], "set") {
		return fmt.Errorf("usage: .mode <show|set>")
	}
	if len(args) < 4 {
		return fmt.Errorf("usage: .mode set <in|out|end|frame|timestamp|timefmt> <value>")
	}

	field := strings.ToLower(args[2])
	value := strings.Join(args[3:], " ")

	switch field {
	case "in":
		d.app.cfg.inputCode = value
	case "out":
		d.app.cfg.outputCode = value
	case "end":
		d.app.cfg.endStr = value
	case "frame":
		n, err := strconv.Atoi(value)
		if err != nil || n <= 0 {
			return fmt.Errorf("frame must be a positive integer")
		}
		d.app.cfg.frameSize = n
	case "timestamp":
		enabled, ok := parseOnOff(value)
		if !ok {
			return fmt.Errorf("timestamp value must be on/off")
		}
		d.app.cfg.timesTamp = enabled
	case "timefmt":
		d.app.cfg.timesFmt = value
	default:
		return fmt.Errorf("unknown mode field: %s", field)
	}

	d.app.Statusf("[mode] %s=%q", field, value)
	return nil
}

func parseOnOff(v string) (bool, bool) {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "on", "true", "1", "yes":
		return true, true
	case "off", "false", "0", "no":
		return false, true
	default:
		return false, false
	}
}
