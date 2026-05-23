package command

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/jixishi/SerialTerminalForWindowsTerminal/internal/event"
	"github.com/jixishi/SerialTerminalForWindowsTerminal/pkg/forward"
)

func (d *Dispatcher) handleForwardCommand(args []string) error {
	if len(args) < 2 {
		if d.host.UIEnabled() {
			d.host.OpenPanel(event.UIPanelForward)
			return nil
		}
		args = []string{".forward", "list"}
	}

	sub := strings.ToLower(args[1])
	switch sub {
	case "list", "stats":
		if d.host.UIEnabled() {
			d.host.OpenPanel(event.UIPanelForward)
			return nil
		}

		items := d.host.Forward().List()
		if len(items) == 0 {
			d.host.Notifyf("[forward] empty")
			return nil
		}
		d.host.Notifyf("[forward] ID Mode Enabled Connected Address InBytes OutBytes LastError")
		for _, it := range items {
			d.host.Notifyf("[forward] %d %s %v %v %s %d %d %s", it.ID, it.Mode, it.Enabled, it.Connected, it.Address, it.ReadBytes, it.WriteByte, it.LastError)
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
		id, err := d.host.Forward().Add(mode, args[3])
		if err != nil {
			return err
		}
		d.host.Statusf("[forward] added #%d", id)
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
			return d.host.Forward().Remove(id)
		case "enable":
			return d.host.Forward().Enable(id)
		case "disable":
			return d.host.Forward().Disable(id)
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
		if err = d.host.Forward().Update(id, mode, args[4]); err != nil {
			return err
		}
		d.host.Statusf("[forward] updated #%d", id)
		return nil
	}

	return fmt.Errorf("unknown subcommand: %s", sub)
}

func (d *Dispatcher) handlePluginCommand(args []string) error {
	if len(args) < 2 {
		if d.host.UIEnabled() {
			d.host.OpenPanel(event.UIPanelPlugin)
			return nil
		}
		args = []string{".plugin", "list"}
	}

	sub := strings.ToLower(args[1])
	switch sub {
	case "list":
		if d.host.UIEnabled() {
			d.host.OpenPanel(event.UIPanelPlugin)
			return nil
		}

		items := d.host.Plugins().List()
		if len(items) == 0 {
			d.host.Notifyf("[plugin] empty")
			return nil
		}
		for _, it := range items {
			d.host.Notifyf("[plugin] %s enabled=%v path=%s", it.Name, it.Enabled, it.Path)
		}
		return nil

	case "load":
		if len(args) < 3 {
			return fmt.Errorf("usage: .plugin load <path>")
		}
		name, err := d.host.Plugins().Load(args[2])
		if err != nil {
			return err
		}
		d.host.Statusf("[plugin] loaded %s", name)
		return nil

	case "unload", "enable", "disable", "reload":
		if len(args) < 3 {
			return fmt.Errorf("usage: .plugin %s <name>", sub)
		}
		name := args[2]
		switch sub {
		case "unload":
			return d.host.Plugins().Unload(name)
		case "enable":
			return d.host.Plugins().Enable(name)
		case "disable":
			return d.host.Plugins().Disable(name)
		case "reload":
			return d.host.Plugins().Reload(name)
		}
	}

	return fmt.Errorf("unknown subcommand: %s", sub)
}

func (d *Dispatcher) handleModeCommand(args []string) error {
	if len(args) < 2 || strings.EqualFold(args[1], "show") {
		if d.host.UIEnabled() {
			d.host.OpenPanel(event.UIPanelMode)
			return nil
		}

		cfg := d.host.Cfg()
		d.host.Notifyf("[mode] input=%s output=%s end=%q hex=%v frame=%d timestamp=%v timefmt=%q forwardTargets=%d plugins=%d",
			cfg.InputCode, cfg.OutputCode, cfg.EndStr,
			strings.EqualFold(cfg.InputCode, "hex"),
			cfg.FrameSize, cfg.TimesTamp, cfg.TimesFmt,
			len(d.host.Forward().List()), len(d.host.Plugins().List()),
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

	cfg := d.host.Cfg()
	switch field {
	case "in":
		if value == "" {
			return fmt.Errorf("input charset must not be empty")
		}
		cfg.InputCode = value
	case "out":
		if value == "" {
			return fmt.Errorf("output charset must not be empty")
		}
		cfg.OutputCode = value
	case "end":
		cfg.EndStr = value
	case "frame":
		n, err := strconv.Atoi(value)
		if err != nil || n <= 0 {
			return fmt.Errorf("frame must be a positive integer")
		}
		cfg.FrameSize = n
	case "timestamp":
		enabled, ok := parseOnOff(value)
		if !ok {
			return fmt.Errorf("timestamp value must be on/off")
		}
		cfg.TimesTamp = enabled
	case "timefmt":
		if value == "" && cfg.TimesTamp {
			return fmt.Errorf("timestamp format must not be empty")
		}
		cfg.TimesFmt = value
	default:
		return fmt.Errorf("unknown mode field: %s", field)
	}

	d.host.Statusf("[mode] %s=%q", field, value)
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
