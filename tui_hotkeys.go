package main

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/jixishi/SerialTerminalForWindowsTerminal/internal/event"
)

func handleLocalHotkey(m *uiModel, key string) bool {
	if m.isLocalHotkey(key, "h") {
		modifier := strings.ToUpper(normalizeHotkeyPrefix(m.app.cfg.HotkeyMod))
		m.app.ShowModal("Shortcuts", modifier+"+C => local exit\nCtrl+C => remote interrupt\n"+modifier+"+F => forward panel\n"+modifier+"+P => plugin panel\n"+modifier+"+M => mode panel\nF1 => shortcut help")
		return true
	}
	if m.isLocalHotkey(key, "f") {
		m.app.OpenPanel(event.UIPanelForward)
		return true
	}
	if m.isLocalHotkey(key, "p") {
		m.app.OpenPanel(event.UIPanelPlugin)
		return true
	}
	if m.isLocalHotkey(key, "m") {
		m.app.OpenPanel(event.UIPanelMode)
		return true
	}
	return false
}

func (m *uiModel) isLocalHotkey(key, action string) bool {
	parts := strings.Split(strings.ToLower(key), "+")
	if len(parts) < 2 || parts[len(parts)-1] != action {
		return false
	}

	hasCtrl := false
	hasAlt := false
	hasShift := false
	for _, p := range parts[:len(parts)-1] {
		switch p {
		case "ctrl":
			hasCtrl = true
		case "alt":
			hasAlt = true
		case "shift":
			hasShift = true
		}
	}

	mod := normalizeHotkeyPrefix(m.app.cfg.HotkeyMod)
	if mod == "ctrl+shift" {
		return hasCtrl && hasShift
	}
	return hasCtrl && hasAlt
}

func normalizeHotkeyPrefix(mod string) string {
	mod = strings.ToLower(strings.TrimSpace(mod))
	if mod != "ctrl+alt" && mod != "ctrl+shift" {
		mod = "ctrl+alt"
	}
	return mod
}

func hotkeyWith(mod, action string) string {
	return normalizeHotkeyPrefix(mod) + "+" + action
}

func parseCtrlKey(key string) (byte, bool) {
	if !strings.HasPrefix(key, "ctrl+") || strings.HasPrefix(key, "ctrl+shift+") {
		return 0, false
	}

	parts := strings.Split(key, "+")
	if len(parts) != 2 || len(parts[1]) != 1 {
		return 0, false
	}
	ch := parts[1][0]
	if ch < 'a' || ch > 'z' {
		return 0, false
	}
	return ch, true
}

func (m *uiModel) handleViewportKey(msg tea.KeyMsg) bool {
	if !m.ready || m.showModal {
		return false
	}

	key := strings.ToLower(msg.String())
	switch key {
	case "pgup", "ctrl+u", "alt+up", "up":
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		_ = cmd
		m.followTail = false
		return true
	case "pgdown", "ctrl+d", "alt+down", "down":
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		_ = cmd
		return true
	case "home", "g":
		m.viewport.GotoTop()
		m.followTail = false
		return true
	case "end", "shift+g":
		m.viewport.GotoBottom()
		m.followTail = true
		return true
	default:
		return false
	}
}

func (m *uiModel) resetCompletion() {
	m.completionActive = false
	m.completionBase = ""
	m.completionCandidates = nil
	m.completionIndex = 0
}

func (m *uiModel) stepCompletion(direction int) {
	if len(m.completionCandidates) == 0 {
		m.resetCompletion()
		return
	}
	if direction >= 0 {
		m.completionIndex = (m.completionIndex + 1) % len(m.completionCandidates)
	} else {
		m.completionIndex = (m.completionIndex - 1 + len(m.completionCandidates)) % len(m.completionCandidates)
	}
	m.applyCompletion()
}

func (m *uiModel) applyCompletion() {
	if len(m.completionCandidates) == 0 {
		return
	}
	m.input.SetValue(m.completionBase + m.completionCandidates[m.completionIndex] + " ")
}

func completionBase(line string) string {
	if strings.HasSuffix(line, " ") {
		return line
	}
	i := strings.LastIndex(line, " ")
	if i < 0 {
		return ""
	}
	return line[:i+1]
}
