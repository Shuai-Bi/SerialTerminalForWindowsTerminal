package termapp

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/jixishi/SerialTerminalForWindowsTerminal/internal/event"
	"github.com/jixishi/SerialTerminalForWindowsTerminal/pkg/forward"
	"github.com/jixishi/SerialTerminalForWindowsTerminal/pkg/luaplugin"
)

type doneMsg struct{}

type modeItem struct {
	key   string
	label string
	value string
}

type panelLine struct {
	text     string
	selected bool
}

type uiModel struct {
	app *App

	viewport viewport.Model
	input    textinput.Model

	ready       bool
	width       int
	height      int
	statusLine  string
	suggestions []string
	content     strings.Builder
	followTail  bool

	showModal  bool
	modalTitle string
	modalBody  string

	panelKind  event.UIPanelKind
	panelIndex int

	forwardItems []forward.Snapshot
	pluginItems  []luaplugin.Snapshot
	modeItems    []modeItem

	promptActive bool
	promptTitle  string
	promptHint   string
	promptInput  textinput.Model
	promptSubmit func(string)

	completionActive     bool
	completionBase       string
	completionCandidates []string
	completionIndex      int
}

func newUIModel(app *App) *uiModel {
	in := textinput.New()
	// bubbles v0.18.0 computes placeholder width using display cells,
	// which can panic on CJK placeholders. Keep this ASCII-only.
	in.Placeholder = "Type to send to remote, use .help for commands"
	in.Focus()
	in.CharLimit = 0
	in.Prompt = "> "
	in.Width = 80

	return &uiModel{app: app, input: in, followTail: true}
}

func (m *uiModel) Init() tea.Cmd {
	return tea.Batch(waitUIEvent(m.app.uiEvents), waitDone(m.app.waitDone()), textinput.Blink)
}

func waitUIEvent(ch <-chan event.UIEvent) tea.Cmd {
	return func() tea.Msg {
		ev, ok := <-ch
		if !ok {
			return doneMsg{}
		}
		return ev
	}
}

func waitDone(ch <-chan struct{}) tea.Cmd {
	return func() tea.Msg {
		<-ch
		return doneMsg{}
	}
}

func (m *uiModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case doneMsg:
		return m, tea.Quit

	case event.UIEvent:
		switch msg.Kind {
		case event.UIEventOutput, event.UIEventStatus:
			if msg.Kind == event.UIEventOutput {
				m.appendOutput(msg.Text)
			} else {
				m.statusLine = msg.Text
			}
		case event.UIEventModal:
			m.showModal = true
			m.panelKind = event.UIPanelNone
			m.modalTitle = msg.Title
			m.modalBody = msg.Text
			m.promptActive = false
		case event.UIEventPanel:
			m.openPanel(msg.Panel)
		}
		return m, waitUIEvent(m.app.uiEvents)

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		inputHeight := 3
		statusHeight := 2
		viewportHeight := msg.Height - inputHeight - statusHeight
		if viewportHeight < 3 {
			viewportHeight = 3
		}

		if !m.ready {
			m.viewport = viewport.New(msg.Width, viewportHeight)
			m.viewport.YPosition = 0
			m.viewport.SetContent(m.content.String())
			m.ready = true
		} else {
			m.viewport.Width = msg.Width
			m.viewport.Height = viewportHeight
		}

		m.input.Width = msg.Width - 4
		m.viewport.GotoBottom()
		m.followTail = true
		return m, nil

	case tea.KeyMsg:
		keyStr := strings.ToLower(msg.String())
		if m.handleViewportKey(msg) {
			return m, nil
		}
		if keyStr != "tab" && keyStr != "shift+tab" {
			m.resetCompletion()
		}

		if m.showModal && m.handleModalKey(msg) {
			return m, nil
		}

		if m.isLocalHotkey(keyStr, "c") {
			m.app.Statusf("[local] exiting by %s+C", strings.ToUpper(normalizeHotkeyPrefix(m.app.cfg.HotkeyMod)))
			m.app.Close()
			return m, tea.Quit
		}

		if handleLocalHotkey(m, keyStr) {
			return m, nil
		}

		// Some terminals can't encode Ctrl+Alt/Shift+H distinctly and report Ctrl+H.
		if keyStr == "ctrl+h" {
			handleLocalHotkey(m, hotkeyWith(m.app.cfg.HotkeyMod, "h"))
			return m, nil
		}

		if letter, ok := parseCtrlKey(keyStr); ok {
			if err := m.app.sendCtrl(letter); err != nil {
				m.app.Notifyf("[remote] ctrl send failed: %v", err)
			}
			return m, nil
		}

		switch keyStr {
		case "f1":
			handleLocalHotkey(m, hotkeyWith(m.app.cfg.HotkeyMod, "h"))
			return m, nil

		case "tab", "shift+tab":
			direction := 1
			if keyStr == "shift+tab" {
				direction = -1
			}

			if m.completionActive && len(m.completionCandidates) > 0 {
				m.stepCompletion(direction)
				return m, nil
			}

			line, cands := m.app.dispatcher.Complete(m.input.Value())
			m.suggestions = cands
			if len(cands) == 0 {
				return m, nil
			}
			if len(cands) == 1 {
				m.input.SetValue(line)
				return m, nil
			}

			m.completionActive = true
			m.completionBase = completionBase(m.input.Value())
			m.completionCandidates = append([]string(nil), cands...)
			if direction < 0 {
				m.completionIndex = len(cands) - 1
			} else {
				m.completionIndex = 0
			}
			m.applyCompletion()
			return m, nil

		case "enter":
			line := m.input.Value()
			m.input.SetValue("")
			m.suggestions = nil
			m.followTail = true
			m.app.handleLine(line)
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m *uiModel) View() string {
	if !m.ready {
		return "Initializing..."
	}

	suggest := "Tab: no candidates"
	if len(m.suggestions) > 1 {
		suggest = "Tab candidates: " + strings.Join(m.suggestions, "  ")
	} else if len(m.suggestions) == 1 {
		suggest = "Tab: " + m.suggestions[0]
	}
	modifier := strings.ToUpper(normalizeHotkeyPrefix(m.app.cfg.HotkeyMod))
	hotkeys := "Hotkeys: Ctrl+C remote | " + modifier + "+C local | " + modifier + "+F forward | " + modifier + "+P plugins | " + modifier + "+M mode | F1 help"
	hotkeys = lipgloss.NewStyle().Faint(true).Foreground(lipgloss.Color("245")).Render(hotkeys)
	status := m.statusLine
	if status == "" {
		status = "Ready"
	}
	status = lipgloss.NewStyle().Foreground(lipgloss.Color("250")).Faint(true).Render(status)
	base := fmt.Sprintf("%s\n%s\n%s\n%s\n%s", m.viewport.View(), suggest, status, m.input.View(), hotkeys)
	if !m.showModal {
		return fillScreen(m.width, m.height, base)
	}

	if m.promptActive {
		return renderCenteredModalContent(m.width, m.height, m.renderPrompt())
	}

	if m.panelKind != event.UIPanelNone {
		return renderCenteredModalContent(m.width, m.height, m.renderPanel())
	}

	return renderCenteredModal(m.width, m.height, m.modalTitle, m.modalBody)
}
