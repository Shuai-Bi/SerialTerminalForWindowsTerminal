package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/jixishi/SerialTerminalForWindowsTerminal/internal/app"
	"github.com/jixishi/SerialTerminalForWindowsTerminal/internal/event"
	"github.com/jixishi/SerialTerminalForWindowsTerminal/pkg/forward"
	"github.com/jixishi/SerialTerminalForWindowsTerminal/pkg/luaplugin"
)

type doneMsg struct{}

type modeItem struct {
	key      string
	label    string
	value    string
	rawValue string
}

type panelLine struct {
	text     string
	selected bool
}

type Model struct {
	App *app.App

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
	panelError string

	forwardItems []forward.Snapshot
	pluginItems  []luaplugin.Snapshot
	modeItems    []modeItem

	promptActive bool
	promptTitle  string
	promptHint   string
	promptInput  textinput.Model
	promptSubmit func(string)

	formActive  bool
	formTitle   string
	formFields  []textinput.Model
	formLabels  []string
	formFocus   int
	formSubmit  func([]string)

	completionActive     bool
	completionBase       string
	completionCandidates []string
	completionIndex      int
}

func New(application *app.App) *Model {
	in := textinput.New()
	in.Placeholder = "Type to send to remote, use .help for commands"
	in.Focus()
	in.CharLimit = 0
	in.Prompt = "> "
	in.Width = 80

	return &Model{App: application, input: in, followTail: true}
}

func (m *Model) Init() tea.Cmd {
	return tea.Batch(waitUIEvent(m.App.UIEvents()), waitDone(m.App.WaitDone()), textinput.Blink)
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

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
		return m, waitUIEvent(m.App.UIEvents())

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

		if m.showModal {
			handled, cmd := m.handleModalKey(msg)
			if handled {
				return m, cmd
			}
		}

		if m.isLocalHotkey(keyStr, "c") {
			m.App.Statusf("[local] exiting by %s+C", strings.ToUpper(normalizeHotkeyPrefix(m.App.Cfg().HotkeyMod)))
			m.App.Close()
			return m, tea.Quit
		}

		if handleLocalHotkey(m, keyStr) {
			return m, nil
		}

		if keyStr == "ctrl+h" {
			handleLocalHotkey(m, hotkeyWith(m.App.Cfg().HotkeyMod, "h"))
			return m, nil
		}

		if letter, ok := parseCtrlKey(keyStr); ok {
			if err := m.App.SendCtrl(letter); err != nil {
				m.App.Notifyf("[remote] ctrl send failed: %v", err)
			}
			return m, nil
		}

		switch keyStr {
		case "f1":
			handleLocalHotkey(m, hotkeyWith(m.App.Cfg().HotkeyMod, "h"))
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

			line, cands := m.App.Dispatcher().Complete(m.input.Value())
			m.suggestions = cands
			if len(cands) == 0 {
				if strings.HasPrefix(strings.TrimSpace(m.input.Value()), ".") {
					data := append([]byte(m.input.Value()), '\t')
					if err := m.App.WriteToSession(data); err != nil {
						m.App.Statusf("[send] %v", err)
					}
					m.input.SetValue("")
				}
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
			m.App.HandleLine(line)
			return m, nil
		}
	}


		// Handle CSI u sequences that bubbletea does not parse into KeyMsg
		if b, ok := msg.([]byte); ok {
			if key, ok2 := parseCSIuBytes(b); ok2 {
				keyStr := strings.ToLower(key)
				if m.showModal {
					last := rune(key[len(key)-1])
					fake := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{last}, Alt: strings.Contains(key, "alt+")}
					if handled, _ := m.handleModalKey(fake); handled {
						return m, nil
					}
				}
				if keyStr == normalizeHotkeyPrefix(m.App.Cfg().HotkeyMod)+"+c" {
					m.App.Close()
					return m, tea.Quit
				}
				if handleLocalHotkey(m, keyStr) {
					return m, nil
				}
			}
		}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m *Model) View() string {
	if !m.ready {
		return "Initializing..."
	}

	suggest := "Tab: no candidates"
	if len(m.suggestions) > 1 {
		suggest = "Tab candidates: " + strings.Join(m.suggestions, "  ")
	} else if len(m.suggestions) == 1 {
		suggest = "Tab: " + m.suggestions[0]
	}
	modifier := strings.ToUpper(normalizeHotkeyPrefix(m.App.Cfg().HotkeyMod))
	hotkeys := "Hotkeys: Ctrl+C remote | " + modifier + "+C local | " + modifier + "+F forward | " + modifier + "+P plugins | " + modifier + "+M mode | F1 help"
	hotkeys = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("244")).Render(hotkeys)
	status := m.statusLine
	if status == "" {
		status = "Ready"
	}
	status = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("255")).Render(status)
	suggest = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("39")).Render(suggest)
	base := fmt.Sprintf("%s\n%s\n%s\n%s\n%s", m.viewport.View(), suggest, status, m.input.View(), hotkeys)
	if !m.showModal {
		return fillScreen(m.width, m.height, base)
	}

	if m.formActive {
		return renderCenteredModalContent(m.width, m.height, m.renderForm())
	}

	if m.promptActive {
		return renderCenteredModalContent(m.width, m.height, m.renderPrompt())
	}

	if m.panelKind != event.UIPanelNone {
		return renderCenteredModalContent(m.width, m.height, m.renderPanel())
	}

	return renderCenteredModal(m.width, m.height, m.modalTitle, m.modalBody)
}
