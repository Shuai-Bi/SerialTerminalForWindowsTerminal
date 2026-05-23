package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/jixishi/SerialTerminalForWindowsTerminal/internal/event"
)

func (m *uiModel) handleModalKey(msg tea.KeyMsg) bool {
	keyStr := strings.ToLower(msg.String())

	if m.promptActive {
		return m.handlePromptKey(msg)
	}
	if keyStr == "esc" {
		m.closeModal()
		return true
	}
	if m.panelKind == event.UIPanelNone {
		if keyStr == "enter" {
			m.closeModal()
		}
		return true
	}

	switch m.panelKind {
	case event.UIPanelForward:
		return m.handleForwardPanelKey(keyStr)
	case event.UIPanelPlugin:
		return m.handlePluginPanelKey(keyStr)
	case event.UIPanelMode:
		return m.handleModePanelKey(keyStr)
	default:
		return true
	}
}

func (m *uiModel) closeModal() {
	m.showModal = false
	m.panelKind = event.UIPanelNone
	m.modalTitle = ""
	m.modalBody = ""
	m.promptActive = false
	m.promptSubmit = nil
}

func (m *uiModel) openPanel(kind event.UIPanelKind) {
	m.showModal = true
	m.panelKind = kind
	m.panelIndex = 0
	m.promptActive = false
	m.promptSubmit = nil
	m.refreshPanel()
}

func (m *uiModel) refreshPanel() {
	switch m.panelKind {
	case event.UIPanelForward:
		m.forwardItems = m.app.forward.List()
		m.panelIndex = clampIndex(m.panelIndex, len(m.forwardItems))
	case event.UIPanelPlugin:
		m.pluginItems = m.app.plugins.List()
		m.panelIndex = clampIndex(m.panelIndex, len(m.pluginItems))
	case event.UIPanelMode:
		m.modeItems = m.buildModeItems()
		m.panelIndex = clampIndex(m.panelIndex, len(m.modeItems))
	}
}

func (m *uiModel) buildModeItems() []modeItem {
	return []modeItem{{"in", "Input Charset", m.app.cfg.InputCode}, {"out", "Output Charset", m.app.cfg.OutputCode}, {"end", "Line End", fmt.Sprintf("%q", m.app.cfg.EndStr)}, {"frame", "Hex Frame Size", fmt.Sprintf("%d", m.app.cfg.FrameSize)}, {"timestamp", "Timestamp", fmt.Sprintf("%v", m.app.cfg.TimesTamp)}, {"timefmt", "Timestamp Format", m.app.cfg.TimesFmt}}
}

func (m *uiModel) handleForwardPanelKey(key string) bool {
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
		m.refreshPanel()
		return true
	case "a":
		m.startPrompt("Add Forward", "tcp 127.0.0.1:12345", "", func(v string) {
			parts := strings.Fields(v)
			if len(parts) < 2 {
				m.app.Statusf("[forward] usage: <tcp|udp> <address>")
				return
			}
			m.app.handleLine(fmt.Sprintf(".forward add %s %s", parts[0], parts[1]))
			m.refreshPanel()
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
			m.app.handleLine(fmt.Sprintf(".forward disable %d", sel.ID))
		} else {
			m.app.handleLine(fmt.Sprintf(".forward enable %d", sel.ID))
		}
		m.refreshPanel()
		return true
	case "d", "delete", "backspace":
		m.app.handleLine(fmt.Sprintf(".forward remove %d", sel.ID))
		m.refreshPanel()
		return true
	case "u":
		m.startPrompt("Update Forward #"+fmt.Sprint(sel.ID), "tcp 127.0.0.1:12345", fmt.Sprintf("%s %s", sel.Mode, sel.Address), func(v string) {
			parts := strings.Fields(v)
			if len(parts) < 2 {
				m.app.Statusf("[forward] usage: <tcp|udp> <address>")
				return
			}
			m.app.handleLine(fmt.Sprintf(".forward update %d %s %s", sel.ID, parts[0], parts[1]))
			m.refreshPanel()
		})
		return true
	default:
		return true
	}
}

func (m *uiModel) handlePluginPanelKey(key string) bool {
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
		m.refreshPanel()
		return true
	case "l":
		m.startPrompt("Load Plugin", "./plugins/demo.lua", "", func(v string) {
			path := strings.TrimSpace(v)
			if path == "" {
				m.app.Statusf("[plugin] load path is empty")
				return
			}
			m.app.handleLine(fmt.Sprintf(".plugin load %s", path))
			m.refreshPanel()
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
			m.app.handleLine(fmt.Sprintf(".plugin disable %s", sel.Name))
		} else {
			m.app.handleLine(fmt.Sprintf(".plugin enable %s", sel.Name))
		}
		m.refreshPanel()
		return true
	case "u":
		m.app.handleLine(fmt.Sprintf(".plugin reload %s", sel.Name))
		m.refreshPanel()
		return true
	case "d", "delete", "backspace":
		m.app.handleLine(fmt.Sprintf(".plugin unload %s", sel.Name))
		m.refreshPanel()
		return true
	default:
		return true
	}
}

func (m *uiModel) handleModePanelKey(key string) bool {
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
		m.refreshPanel()
		return true
	}
	if len(m.modeItems) == 0 {
		return true
	}

	sel := m.modeItems[m.panelIndex]
	switch key {
	case " ":
		if sel.key == "timestamp" {
			if m.app.cfg.TimesTamp {
				m.app.handleLine(".mode set timestamp off")
			} else {
				m.app.handleLine(".mode set timestamp on")
			}
			m.refreshPanel()
		}
		return true
	case "enter", "e":
		initial := strings.Trim(sel.value, "\"")
		m.startPrompt("Edit Mode: "+sel.label, "new value", initial, func(v string) {
			m.app.handleLine(fmt.Sprintf(".mode set %s %s", sel.key, v))
			m.refreshPanel()
		})
		return true
	default:
		return true
	}
}

func (m *uiModel) startPrompt(title, hint, initial string, submit func(string)) {
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

func (m *uiModel) handlePromptKey(msg tea.KeyMsg) bool {
	key := strings.ToLower(msg.String())
	switch key {
	case "esc":
		m.promptActive = false
		m.promptSubmit = nil
		return true
	case "enter":
		value := strings.TrimSpace(m.promptInput.Value())
		submit := m.promptSubmit
		m.promptActive = false
		m.promptSubmit = nil
		if submit != nil {
			submit(value)
		}
		return true
	default:
		var cmd tea.Cmd
		m.promptInput, cmd = m.promptInput.Update(msg)
		_ = cmd
		return true
	}
}

func (m *uiModel) renderPanel() string {
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

func (m *uiModel) renderForwardPanel() string {
	lines := make([]panelLine, 0, len(m.forwardItems)+2)
	if len(m.forwardItems) == 0 {
		lines = append(lines, panelLine{text: "No forwarding targets. Press 'a' to add one."})
	} else {
		lines = append(lines, panelLine{text: "ID  Mode  Enabled  Connected  Address                InBytes  OutBytes"})
		for i, it := range m.forwardItems {
			lines = append(lines, panelLine{text: fmt.Sprintf("%-3d %-5s %-7v %-9v %-22s %-7d %-8d", it.ID, it.Mode, it.Enabled, it.Connected, it.Address, it.ReadBytes, it.WriteByte), selected: i == m.panelIndex})
		}
	}
	return renderPanelModal("Forward Panel", lines, "Up/Down select | Enter toggle enable | a add | u update | d remove | r refresh | Esc close", m.availableModalWidth())
}

func (m *uiModel) renderPluginPanel() string {
	lines := make([]panelLine, 0, len(m.pluginItems)+2)
	if len(m.pluginItems) == 0 {
		lines = append(lines, panelLine{text: "No plugins loaded. Press 'l' to load one."})
	} else {
		lines = append(lines, panelLine{text: "Name                 Enabled  Path"})
		for i, it := range m.pluginItems {
			lines = append(lines, panelLine{text: fmt.Sprintf("%-20s %-7v %s", it.Name, it.Enabled, it.Path), selected: i == m.panelIndex})
		}
	}
	return renderPanelModal("Plugin Panel", lines, "Up/Down select | Enter toggle enable | l load | u reload | d unload | r refresh | Esc close", m.availableModalWidth())
}

func (m *uiModel) renderModePanel() string {
	lines := make([]panelLine, 0, len(m.modeItems)+2)
	lines = append(lines, panelLine{text: "Field            Value"})
	for i, it := range m.modeItems {
		lines = append(lines, panelLine{text: fmt.Sprintf("%-16s %s", it.label, it.value), selected: i == m.panelIndex})
	}
	return renderPanelModal("Mode Panel", lines, "Up/Down select | Enter edit value | Space toggle timestamp | r refresh | Esc close", m.availableModalWidth())
}
