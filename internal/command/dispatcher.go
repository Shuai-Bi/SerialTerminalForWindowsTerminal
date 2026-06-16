package command

import (
	"encoding/hex"
	"fmt"
	"sort"
	"strings"

	"github.com/jixishi/SerialTerminalForWindowsTerminal/internal/config"
	"github.com/jixishi/SerialTerminalForWindowsTerminal/internal/event"
	"github.com/jixishi/SerialTerminalForWindowsTerminal/pkg/forward"
	"github.com/jixishi/SerialTerminalForWindowsTerminal/pkg/luaplugin"
)

// CommandHost is the minimal interface the command dispatcher needs from its host.
type CommandHost interface {
	Close()
	Notifyf(format string, args ...any)
	Statusf(format string, args ...any)
	ShowModal(title, text string)
	OpenPanel(panel event.UIPanelKind)
	UIEnabled() bool
	WriteToSession(data []byte) error
	Forward() *forward.Manager
	Plugins() *luaplugin.Manager
	Cfg() *config.Config
}

type CommandHandler func(args []string) error
type CommandCompleter func(args []string) []string

type RuntimeCommand struct {
	Name        string
	Usage       string
	Description string
	Handler     CommandHandler
	Completer   CommandCompleter
}

type Dispatcher struct {
	host     CommandHost
	commands map[string]*RuntimeCommand
	order    []string
}

func NewDispatcher(host CommandHost) *Dispatcher {
	d := &Dispatcher{
		host:     host,
		commands: make(map[string]*RuntimeCommand),
	}
	d.registerAll()
	return d
}

func (d *Dispatcher) register(cmd RuntimeCommand) {
	key := strings.ToLower(cmd.Name)
	d.commands[key] = &cmd
	d.order = append(d.order, key)
}

func (d *Dispatcher) registerAll() {
	d.register(RuntimeCommand{
		Name:        ".help",
		Usage:       ".help",
		Description: "show command help",
		Handler: func(args []string) error {
			d.host.ShowModal("Command Help", d.HelpText())
			return nil
		},
	})

	d.register(RuntimeCommand{
		Name:        ".exit",
		Usage:       ".exit",
		Description: "exit local terminal",
		Handler: func(args []string) error {
			d.host.Statusf("[local] exiting")
			d.host.Close()
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
			return d.host.WriteToSession(b)
		},
	})

	d.register(RuntimeCommand{
		Name:        ".forward",
		Usage:       ".forward <list|add|remove|enable|disable|update>",
		Description: "manage forwarding (tcp/udp/tcp-s/udp-s/com)",
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

func (d *Dispatcher) Execute(line string) (bool, error) {
	args := strings.Fields(strings.TrimSpace(line))
	if len(args) == 0 {
		return false, nil
	}
	if !strings.HasPrefix(args[0], ".") {
		return false, nil
	}

	cmd, ok := d.commands[strings.ToLower(args[0])]
	if !ok {
		return false, nil
	}

	if err := cmd.Handler(args); err != nil {
		return true, err
	}
	return true, nil
}

func (d *Dispatcher) HelpText() string {
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

func (d *Dispatcher) Complete(line string) (string, []string) {
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

func (d *Dispatcher) commandNames() []string {
	names := make([]string, 0, len(d.commands))
	for _, cmd := range d.commands {
		names = append(names, cmd.Name)
	}
	sort.Strings(names)
	return names
}
