package main

import (
	"io"
	"strings"
	"testing"

	"github.com/jixishi/SerialTerminalForWindowsTerminal/internal/event"
	"github.com/jixishi/SerialTerminalForWindowsTerminal/pkg/forward"
	"github.com/jixishi/SerialTerminalForWindowsTerminal/pkg/luaplugin"
)

func setupTestPipes() {
	var cr *io.PipeReader
	cr, stdinPipe = io.Pipe()
	go func() {
		buf := make([]byte, 4096)
		for {
			_, err := cr.Read(buf)
			if err != nil {
				return
			}
		}
	}()
}

func newTestAppForCommand() *App {
	a := &App{
		cfg:      &Config{inputCode: "UTF-8", outputCode: "UTF-8", endStr: "\n"},
		plugins:  luaplugin.NewManager(),
		uiEvents: make(chan event.UIEvent, 32),
		done:     make(chan struct{}),
	}
	a.SetUIEnabled(true)
	a.forward = forward.NewManager(func([]byte) error { return nil }, func(string, ...any) {})
	a.dispatcher = NewCommandDispatcher(a)
	return a
}

func TestCommandCompleteRoot(t *testing.T) {
	a := newTestAppForCommand()
	line, cands := a.dispatcher.Complete(".")
	if line != "." {
		t.Fatalf("expected line unchanged for ambiguous completion, got %q", line)
	}
	if len(cands) == 0 {
		t.Fatalf("expected root command candidates")
	}
	for _, c := range cands {
		if c == ".ctrl" {
			t.Fatalf(".ctrl should be removed from command set")
		}
	}
}

func TestCommandCompleteForwardSubcommands(t *testing.T) {
	a := newTestAppForCommand()
	_, cands := a.dispatcher.Complete(".forward ")
	joined := strings.Join(cands, ",")
	for _, name := range []string{"list", "add", "remove", "enable", "disable", "update", "stats"} {
		if !strings.Contains(joined, name) {
			t.Fatalf("missing forward candidate %q in %v", name, cands)
		}
	}
}

func TestCommandExecuteUnknown(t *testing.T) {
	a := newTestAppForCommand()
	handled, err := a.dispatcher.Execute(".unknown")
	if !handled {
		t.Fatalf("unknown command should be marked handled")
	}
	if err == nil {
		t.Fatalf("expected unknown command error")
	}
}

func TestCommandExecuteHelpShowsModal(t *testing.T) {
	a := newTestAppForCommand()
	handled, err := a.dispatcher.Execute(".help")
	if err != nil || !handled {
		t.Fatalf(".help execute failed handled=%v err=%v", handled, err)
	}

	ev := mustReadEvent(t, a.uiEvents)
	if ev.Kind != event.UIEventModal || ev.Title == "" {
		t.Fatalf("expected help modal event, got %+v", ev)
	}
}

func TestCommandExecuteForwardListShowsPanel(t *testing.T) {
	a := newTestAppForCommand()
	handled, err := a.dispatcher.Execute(".forward list")
	if err != nil || !handled {
		t.Fatalf(".forward list execute failed handled=%v err=%v", handled, err)
	}

	ev := mustReadEvent(t, a.uiEvents)
	if ev.Kind != event.UIEventPanel || ev.Panel != event.UIPanelForward {
		t.Fatalf("expected forward panel event, got %+v", ev)
	}
}

func TestCommandExecutePluginListShowsPanel(t *testing.T) {
	a := newTestAppForCommand()
	if _, err := a.plugins.Load("plugins/demo.lua"); err == nil {
		_ = a.plugins.Disable("demo")
	}
	handled, err := a.dispatcher.Execute(".plugin list")
	if err != nil || !handled {
		t.Fatalf(".plugin list execute failed handled=%v err=%v", handled, err)
	}

	ev := mustReadEvent(t, a.uiEvents)
	if ev.Kind != event.UIEventPanel || ev.Panel != event.UIPanelPlugin {
		t.Fatalf("expected plugin panel event, got %+v", ev)
	}
}

func TestCommandExecutePluginWithoutSubcommandShowsPanel(t *testing.T) {
	a := newTestAppForCommand()
	handled, err := a.dispatcher.Execute(".plugin")
	if err != nil || !handled {
		t.Fatalf(".plugin execute failed handled=%v err=%v", handled, err)
	}

	ev := mustReadEvent(t, a.uiEvents)
	if ev.Kind != event.UIEventPanel || ev.Panel != event.UIPanelPlugin {
		t.Fatalf("expected plugin panel event for bare command, got %+v", ev)
	}
}

func TestCommandExecuteModeShowsPanel(t *testing.T) {
	a := newTestAppForCommand()
	handled, err := a.dispatcher.Execute(".mode show")
	if err != nil || !handled {
		t.Fatalf(".mode execute failed handled=%v err=%v", handled, err)
	}

	ev := mustReadEvent(t, a.uiEvents)
	if ev.Kind != event.UIEventPanel || ev.Panel != event.UIPanelMode {
		t.Fatalf("expected mode panel event, got %+v", ev)
	}
}

func TestCommandExecuteModeSet(t *testing.T) {
	a := newTestAppForCommand()
	handled, err := a.dispatcher.Execute(".mode set end \\r\\n")
	if err != nil || !handled {
		t.Fatalf(".mode set end failed handled=%v err=%v", handled, err)
	}
	if a.cfg.endStr != "\\r\\n" {
		t.Fatalf("mode set end not applied, got=%q", a.cfg.endStr)
	}

	handled, err = a.dispatcher.Execute(".mode set timestamp on")
	if err != nil || !handled {
		t.Fatalf(".mode set timestamp failed handled=%v err=%v", handled, err)
	}
	if !a.cfg.timesTamp {
		t.Fatalf("mode set timestamp should enable timesTamp")
	}
}

func TestParseOnOff(t *testing.T) {
	tests := []struct {
		in    string
		val   bool
		valid bool
	}{
		{in: "on", val: true, valid: true},
		{in: "true", val: true, valid: true},
		{in: "1", val: true, valid: true},
		{in: "yes", val: true, valid: true},
		{in: "off", val: false, valid: true},
		{in: "false", val: false, valid: true},
		{in: "0", val: false, valid: true},
		{in: "no", val: false, valid: true},
		{in: "", val: false, valid: false},
		{in: "maybe", val: false, valid: false},
	}

	for _, tt := range tests {
		got, ok := parseOnOff(tt.in)
		if ok != tt.valid || got != tt.val {
			t.Fatalf("parseOnOff(%q) got=(%v,%v) want=(%v,%v)", tt.in, got, ok, tt.val, tt.valid)
		}
	}
}

func TestCompleteForward(t *testing.T) {
	tests := []struct {
		args []string
		want []string
	}{
		{args: []string{".forward"}, want: []string{"list", "add", "remove", "enable", "disable", "update", "stats"}},
		{args: []string{".forward", ""}, want: []string{"list", "add", "remove", "enable", "disable", "update", "stats"}},
		{args: []string{".forward", "add", ""}, want: []string{"tcp", "udp"}},
		{args: []string{".forward", "update", "1", ""}, want: []string{"tcp", "udp"}},
		{args: []string{".forward", "list", "1"}, want: nil},
	}
	for _, tt := range tests {
		got := completeForward(tt.args)
		if !stringSlicesEqual(got, tt.want) {
			t.Fatalf("completeForward(%v) got=%v want=%v", tt.args, got, tt.want)
		}
	}
}

func TestCompletePlugin(t *testing.T) {
	tests := []struct {
		args []string
		want []string
	}{
		{args: []string{".plugin"}, want: []string{"list", "load", "unload", "enable", "disable", "reload"}},
		{args: []string{".plugin", "load", ""}, want: nil},
		{args: []string{".plugin", "unload", "demo"}, want: nil},
	}
	for _, tt := range tests {
		got := completePlugin(tt.args)
		if !stringSlicesEqual(got, tt.want) {
			t.Fatalf("completePlugin(%v) got=%v want=%v", tt.args, got, tt.want)
		}
	}
}

func TestCompleteMode(t *testing.T) {
	tests := []struct {
		args []string
		want []string
	}{
		{args: []string{".mode"}, want: []string{"show", "set"}},
		{args: []string{".mode", "set", ""}, want: []string{"in", "out", "end", "frame", "timestamp", "timefmt"}},
		{args: []string{".mode", "set", "timestamp", ""}, want: []string{"on", "off"}},
		{args: []string{".mode", "set", "in", ""}, want: nil},
	}
	for _, tt := range tests {
		got := completeMode(tt.args)
		if !stringSlicesEqual(got, tt.want) {
			t.Fatalf("completeMode(%v) got=%v want=%v", tt.args, got, tt.want)
		}
	}
}

func stringSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestHelpText(t *testing.T) {
	a := newTestAppForCommand()
	text := a.dispatcher.HelpText()
	for _, cmd := range []string{".help", ".exit", ".hex", ".forward", ".plugin", ".mode"} {
		if !strings.Contains(text, cmd) {
			t.Fatalf("HelpText missing command %q", cmd)
		}
	}
}

func TestCommandExecuteHex(t *testing.T) {
	setupTestPipes()
	a := newTestAppForCommand()
	handled, err := a.dispatcher.Execute(".hex 41 42 43")
	if err != nil || !handled {
		t.Fatalf(".hex valid failed handled=%v err=%v", handled, err)
	}

	handled, err = a.dispatcher.Execute(".hex")
	if !handled || err == nil {
		t.Fatalf(".hex no args should error, handled=%v err=%v", handled, err)
	}

	handled, err = a.dispatcher.Execute(".hex xyz")
	if !handled || err == nil {
		t.Fatalf(".hex invalid hex should error, handled=%v err=%v", handled, err)
	}
}

func TestCommandExecuteExit(t *testing.T) {
	a := newTestAppForCommand()
	a.Close()
	if !a.isClosed() {
		t.Fatalf("expected app closed after Close()")
	}
}

func TestCommandExecuteModeSetAll(t *testing.T) {
	a := newTestAppForCommand()

	handled, err := a.dispatcher.Execute(".mode set frame 32")
	if err != nil || !handled {
		t.Fatalf(".mode set frame failed: handled=%v err=%v", handled, err)
	}
	if a.cfg.frameSize != 32 {
		t.Fatalf("frameSize not set, got=%d", a.cfg.frameSize)
	}

	handled, err = a.dispatcher.Execute(".mode set timefmt 2006")
	if err != nil || !handled {
		t.Fatalf(".mode set timefmt failed: handled=%v err=%v", handled, err)
	}
	if a.cfg.timesFmt != "2006" {
		t.Fatalf("timesFmt not set, got=%q", a.cfg.timesFmt)
	}

	handled, err = a.dispatcher.Execute(".mode set out GBK")
	if err != nil || !handled {
		t.Fatalf(".mode set out failed: handled=%v err=%v", handled, err)
	}
	if a.cfg.outputCode != "GBK" {
		t.Fatalf("outputCode not set, got=%q", a.cfg.outputCode)
	}

	handled, err = a.dispatcher.Execute(".mode set in GBK")
	if err != nil || !handled {
		t.Fatalf(".mode set in failed: handled=%v err=%v", handled, err)
	}
	if a.cfg.inputCode != "GBK" {
		t.Fatalf("inputCode not set, got=%q", a.cfg.inputCode)
	}
}

func TestCommandExecuteModeErrors(t *testing.T) {
	a := newTestAppForCommand()

	handled, err := a.dispatcher.Execute(".mode")
	if err != nil || !handled {
		t.Fatalf(".mode with no subcommand in UI mode shows panel, handled=%v err=%v", handled, err)
	}

	_, err = a.dispatcher.Execute(".mode set")
	if err == nil {
		t.Fatalf(".mode set with no args should error")
	}

	_, err = a.dispatcher.Execute(".mode set frame abc")
	if err == nil {
		t.Fatalf(".mode set frame with non-int should error")
	}

	_, err = a.dispatcher.Execute(".mode set timestamp maybe")
	if err == nil {
		t.Fatalf(".mode set timestamp with invalid value should error")
	}

	_, err = a.dispatcher.Execute(".mode set invalid_field value")
	if err == nil {
		t.Fatalf(".mode set unknown field should error")
	}
}

func TestHandleForwardCommandErrors(t *testing.T) {
	a := newTestAppForCommand()

	_, err := a.dispatcher.Execute(".forward add")
	if err == nil {
		t.Fatalf(".forward add with no args should error")
	}

	_, err = a.dispatcher.Execute(".forward add badmode 127.0.0.1:1")
	if err == nil {
		t.Fatalf(".forward add with invalid mode should error")
	}

	_, err = a.dispatcher.Execute(".forward remove abc")
	if err == nil {
		t.Fatalf(".forward remove with non-int ID should error")
	}

	_, err = a.dispatcher.Execute(".forward remove 999")
	if err == nil {
		t.Fatalf(".forward remove non-existing should error")
	}

	_, err = a.dispatcher.Execute(".forward enable abc")
	if err == nil {
		t.Fatalf(".forward enable with non-int ID should error")
	}

	_, err = a.dispatcher.Execute(".forward disable abc")
	if err == nil {
		t.Fatalf(".forward disable with non-int ID should error")
	}

	_, err = a.dispatcher.Execute(".forward update")
	if err == nil {
		t.Fatalf(".forward update with no args should error")
	}

	_, err = a.dispatcher.Execute(".forward update 1")
	if err == nil {
		t.Fatalf(".forward update with missing addr should error")
	}

	_, err = a.dispatcher.Execute(".forward update 1 badmode 127.0.0.1:1")
	if err == nil {
		t.Fatalf(".forward update with invalid mode should error")
	}

	_, err = a.dispatcher.Execute(".forward unknown_sub")
	if err == nil {
		t.Fatalf(".forward unknown subcommand should error")
	}
}

func TestHandleForwardCommandNoUI(t *testing.T) {
	a := newTestAppForCommand()
	a.SetUIEnabled(false)

	handled, err := a.dispatcher.Execute(".forward")
	if err != nil || !handled {
		t.Fatalf(".forward in non-UI should default to list, handled=%v err=%v", handled, err)
	}

	handled, err = a.dispatcher.Execute(".forward list")
	if err != nil || !handled {
		t.Fatalf(".forward list in non-UI failed: %v", err)
	}
}

func TestHandlePluginCommandErrors(t *testing.T) {
	a := newTestAppForCommand()

	_, err := a.dispatcher.Execute(".plugin load")
	if err == nil {
		t.Fatalf(".plugin load with no path should error")
	}

	_, err = a.dispatcher.Execute(".plugin unload")
	if err == nil {
		t.Fatalf(".plugin unload with no name should error")
	}

	_, err = a.dispatcher.Execute(".plugin enable")
	if err == nil {
		t.Fatalf(".plugin enable with no name should error")
	}

	_, err = a.dispatcher.Execute(".plugin disable")
	if err == nil {
		t.Fatalf(".plugin disable with no name should error")
	}

	_, err = a.dispatcher.Execute(".plugin reload")
	if err == nil {
		t.Fatalf(".plugin reload with no name should error")
	}

	_, err = a.dispatcher.Execute(".plugin unknown_sub")
	if err == nil {
		t.Fatalf(".plugin unknown subcommand should error")
	}
}

func TestHandlePluginCommandNoUI(t *testing.T) {
	a := newTestAppForCommand()
	a.SetUIEnabled(false)

	handled, err := a.dispatcher.Execute(".plugin")
	if err != nil || !handled {
		t.Fatalf(".plugin in non-UI should default to list, handled=%v err=%v", handled, err)
	}
}

func TestCompleteFirstTokenEdgeCases(t *testing.T) {
	a := newTestAppForCommand()
	line, cands := a.dispatcher.Complete(".he")
	if line != ".he" {
		t.Fatalf("ambiguous completion should not change line, got=%q", line)
	}
	found := false
	for _, c := range cands {
		if c == ".help" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected .help in completion candidates, got %v", cands)
	}

	line, cands = a.dispatcher.Complete(".exi")
	if line != ".exit " || len(cands) != 1 || cands[0] != ".exit" {
		t.Fatalf("exact completion of .exi failed: line=%q cands=%v", line, cands)
	}

	line, _ = a.dispatcher.Complete("")
	if line != "" {
		t.Fatalf("empty completion should be noop, got=%q", line)
	}
}
