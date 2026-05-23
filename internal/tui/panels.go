package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/jixishi/SerialTerminalForWindowsTerminal/internal/event"
	"github.com/jixishi/SerialTerminalForWindowsTerminal/pkg/forward"
)

func (m *Model) handleModalKey(msg tea.KeyMsg) (bool, tea.Cmd) {
	keyStr := strings.ToLower(msg.String())

	if m.promptActive {
		return m.handlePromptKey(msg)
	}
	if keyStr == "esc" {
		m.closeModal()
		return true, nil
	}
	if m.panelKind == event.UIPanelNone {
		if keyStr == "enter" {
			m.closeModal()
		}
		return true, nil
	}

	switch m.panelKind {
	case event.UIPanelForward:
		return m.handleForwardPanelKey(keyStr), nil
	case event.UIPanelPlugin:
		return m.handlePluginPanelKey(keyStr), nil
	case event.UIPanelMode:
		return m.handleModePanelKey(keyStr), nil
	default:
		return true, nil
	}
}

func (m *Model) closeModal() {
	m.showModal = false
	m.panelKind = event.UIPanelNone
	m.modalTitle = ""
	m.modalBody = ""
	m.promptActive = false
	m.promptSubmit = nil
	m.panelError = ""
}

func (m *Model) openPanel(kind event.UIPanelKind) {
	m.showModal = true
	m.panelKind = kind
	m.panelIndex = 0
	m.promptActive = false
	m.promptSubmit = nil
	m.panelError = ""
	m.refreshPanel()
}

func (m *Model) refreshPanel() {
	switch m.panelKind {
	case event.UIPanelForward:
		m.forwardItems = m.App.Forward().List()
		m.panelIndex = clampIndex(m.panelIndex, len(m.forwardItems))
	case event.UIPanelPlugin:
		m.pluginItems = m.App.Plugins().List()
		m.panelIndex = clampIndex(m.panelIndex, len(m.pluginItems))
	case event.UIPanelMode:
		m.modeItems = m.buildModeItems()
		m.panelIndex = clampIndex(m.panelIndex, len(m.modeItems))
	}
}

func (m *Model) buildModeItems() []modeItem {
	cfg := m.App.Cfg()
	return []modeItem{
		{"in", "Input Charset", cfg.InputCode, cfg.InputCode},
		{"out", "Output Charset", cfg.OutputCode, cfg.OutputCode},
		{"end", "Line End", fmt.Sprintf("%q", cfg.EndStr), cfg.EndStr},
		{"frame", "Hex Frame Size", fmt.Sprintf("%d", cfg.FrameSize), fmt.Sprintf("%d", cfg.FrameSize)},
		{"timestamp", "Timestamp", fmt.Sprintf("%v", cfg.TimesTamp), fmt.Sprintf("%v", cfg.TimesTamp)},
		{"timefmt", "Timestamp Format", cfg.TimesFmt, cfg.TimesFmt},
	}
}

func (m *Model) handleForwardPanelKey(key string) bool {
	switch key {
	case "up", "k":
		if m.panelIndex > 0 {
			m.panelIndex--
		}
		return true
	case "down", "j":
		if m.panelIndex < len(m.forwardItems)-1 {
			m.panelIndex++
		}
		return true
	case "r":
		m.panelError = ""
		m.refreshPanel()
		return true
	case "a":
		m.startPrompt("Add Forward", "tcp 127.0.0.1:12345", "", func(v string) {
			parts := strings.Fields(v)
			if len(parts) < 2 {
				m.panelError = "usage: <tcp|udp> <address>"
				return
			}
			mode, ok := forward.ParseMode(parts[0])
			if !ok {
				m.panelError = "unknown mode: " + parts[0]
				return
			}
			if _, err := m.App.Forward().Add(mode, parts[1]); err != nil {
				m.panelError = err.Error()
			} else {
				m.panelError = ""
				m.refreshPanel()
			}
		})
		return true
	}
	if len(m.forwardItems) == 0 {
		return true
	}

	sel := m.forwardItems[m.panelIndex]
	switch key {
	case "enter":
		if sel.Enabled {
			if err := m.App.Forward().Disable(sel.ID); err != nil {
				m.panelError = err.Error()
			}
		} else {
			if err := m.App.Forward().Enable(sel.ID); err != nil {
				m.panelError = err.Error()
			}
		}
		m.panelError = ""
		m.refreshPanel()
		return true
	case "d", "delete":
		m.startPrompt("Remove Forward #"+fmt.Sprint(sel.ID), "type 'y' to confirm", "", func(v string) {
			if strings.TrimSpace(strings.ToLower(v)) == "y" {
				if err := m.App.Forward().Remove(sel.ID); err != nil {
					m.panelError = err.Error()
				} else {
					m.panelError = ""
					m.refreshPanel()
				}
			}
		})
		return true
	case "u":
		m.startPrompt("Update Forward #"+fmt.Sprint(sel.ID), "tcp 127.0.0.1:12345", fmt.Sprintf("%s %s", sel.Mode, sel.Address), func(v string) {
			parts := strings.Fields(v)
			if len(parts) < 2 {
				m.panelError = "usage: <tcp|udp> <address>"
				return
			}
			mode, ok := forward.ParseMode(parts[0])
			if !ok {
				m.panelError = "unknown mode: " + parts[0]
				return
			}
			if err := m.App.Forward().Update(sel.ID, mode, parts[1]); err != nil {
				m.panelError = err.Error()
			} else {
				m.panelError = ""
				m.refreshPanel()
			}
		})
		return true
	default:
		return true
	}
}

func (m *Model) handlePluginPanelKey(key string) bool {
	switch key {
	case "up", "k":
		if m.panelIndex > 0 {
			m.panelIndex--
		}
		return true
	case "down", "j":
		if m.panelIndex < len(m.pluginItems)-1 {
			m.panelIndex++
		}
		return true
	case "r":
		m.panelError = ""
		m.refreshPanel()
		return true
	case "l":
		m.startPrompt("Load Plugin", "./plugins/demo.lua", "", func(v string) {
			path := strings.TrimSpace(v)
			if path == "" {
				m.panelError = "load path is empty"
				return
			}
			if _, err := m.App.Plugins().Load(path); err != nil {
				m.panelError = err.Error()
			} else {
				m.panelError = ""
				m.refreshPanel()
			}
		})
		return true
	}
	if len(m.pluginItems) == 0 {
		return true
	}

	sel := m.pluginItems[m.panelIndex]
	switch key {
	case "enter":
		if sel.Enabled {
			_ = m.App.Plugins().Disable(sel.Name)
		} else {
			_ = m.App.Plugins().Enable(sel.Name)
		}
		m.panelError = ""
		m.refreshPanel()
		return true
	case "u":
		if err := m.App.Plugins().Reload(sel.Name); err != nil {
			m.panelError = err.Error()
		} else {
			m.panelError = ""
			m.refreshPanel()
		}
		return true
	case "d", "delete":
		m.startPrompt("Unload Plugin "+sel.Name, "type 'y' to confirm", "", func(v string) {
			if strings.TrimSpace(strings.ToLower(v)) == "y" {
				if err := m.App.Plugins().Unload(sel.Name); err != nil {
					m.panelError = err.Error()
				} else {
					m.panelError = ""
					m.refreshPanel()
				}
			}
		})
		return true
	default:
		return true
	}
}

func (m *Model) handleModePanelKey(key string) bool {
	switch key {
	case "up", "k":
		if m.panelIndex > 0 {
			m.panelIndex--
		}
		return true
	case "down", "j":
		if m.panelIndex < len(m.modeItems)-1 {
			m.panelIndex++
		}
		return true
	case "r":
		m.panelError = ""
		m.refreshPanel()
		return true
	}
	if len(m.modeItems) == 0 {
		return true
	}

	sel := m.modeItems[m.panelIndex]
	cfg := m.App.Cfg()
	switch key {
	case " ":
		if sel.key == "timestamp" {
			cfg.TimesTamp = !cfg.TimesTamp
			m.refreshPanel()
		}
		return true
	case "enter", "e":
		hint := "enter value"
		switch sel.key {
		case "timestamp":
			hint = "on/off"
		case "frame":
			hint = "positive integer"
		case "in", "out":
			hint = "charset name (e.g. utf-8, gbk)"
		}
		initial := sel.rawValue
		m.startPrompt("Edit Mode: "+sel.label, hint, initial, func(v string) {
			m.App.HandleLine(fmt.Sprintf(".mode set %s %s", sel.key, v))
			m.refreshPanel()
		})
		return true
	default:
		return true
	}
}

func (m *Model) startPrompt(title, hint, initial string, submit func(string)) {
	in := textinput.New()
	in.Prompt = "> "
	in.Placeholder = hint
	in.SetValue(initial)
	in.Focus()
	in.CharLimit = 0
	in.Width = 64

	m.promptActive = true
	m.promptTitle = title
	m.promptHint = hint
	m.promptInput = in
	m.promptSubmit = submit
}

func (m *Model) handlePromptKey(msg tea.KeyMsg) (bool, tea.Cmd) {
	key := strings.ToLower(msg.String())
	switch key {
	case "esc":
		m.promptActive = false
		m.promptSubmit = nil
		return true, nil
	case "enter":
		value := strings.TrimSpace(m.promptInput.Value())
		submit := m.promptSubmit
		m.promptActive = false
		m.promptSubmit = nil
		if submit != nil {
			submit(value)
		}
		return true, nil
	default:
		var cmd tea.Cmd
		m.promptInput, cmd = m.promptInput.Update(msg)
		return true, cmd
	}
}

func (m *Model) renderPanel() string {
	switch m.panelKind {
	case event.UIPanelForward:
		return m.renderForwardPanel()
	case event.UIPanelPlugin:
		return m.renderPluginPanel()
	case event.UIPanelMode:
		return m.renderModePanel()
	default:
		return renderModal("Info", "No panel", m.availableModalWidth())
	}
}

func (m *Model) renderForwardPanel() string {
	lines := make([]panelLine, 0, len(m.forwardItems)+3)
	if len(m.forwardItems) == 0 {
		lines = append(lines, panelLine{text: "No forwarding targets. Press 'a' to add one."})
	} else {
		lines = append(lines, panelLine{text: "ID  Mode  Enabled  Connected  Address"})
		for i, it := range m.forwardItems {
			lines = append(lines, panelLine{text: fmt.Sprintf("%-3d %-5s %-7v %-9v %s", it.ID, it.Mode, it.Enabled, it.Connected, it.Address), selected: i == m.panelIndex})
		}
	}
	if m.panelError != "" {
		lines = append(lines, panelLine{text: "ERROR: " + m.panelError})
	}
	return renderPanelModal("Forward Panel", lines, "Up/Down select | Enter toggle | a add | u update | d remove | r refresh | Esc close", m.availableModalWidth())
}

func (m *Model) renderPluginPanel() string {
	lines := make([]panelLine, 0, len(m.pluginItems)+3)
	if len(m.pluginItems) == 0 {
		lines = append(lines, panelLine{text: "No plugins loaded. Press 'l' to load one."})
	} else {
		lines = append(lines, panelLine{text: "Name                 Enabled  Path"})
		for i, it := range m.pluginItems {
			lines = append(lines, panelLine{text: fmt.Sprintf("%-20s %-7v %s", it.Name, it.Enabled, it.Path), selected: i == m.panelIndex})
		}
	}
	if m.panelError != "" {
		lines = append(lines, panelLine{text: "ERROR: " + m.panelError})
	}
	return renderPanelModal("Plugin Panel", lines, "Up/Down select | Enter toggle | l load | u reload | d unload | r refresh | Esc close", m.availableModalWidth())
}

func (m *Model) renderModePanel() string {
	lines := make([]panelLine, 0, len(m.modeItems)+3)
	lines = append(lines, panelLine{text: "Field            Value"})
	for i, it := range m.modeItems {
		lines = append(lines, panelLine{text: fmt.Sprintf("%-16s %s", it.label, it.value), selected: i == m.panelIndex})
	}
	if m.panelError != "" {
		lines = append(lines, panelLine{text: "ERROR: " + m.panelError})
	}
	return renderPanelModal("Mode Panel", lines, "Up/Down select | Enter edit | Space toggle | r refresh | Esc close", m.availableModalWidth())
}

