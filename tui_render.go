package main

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func (m *uiModel) appendOutput(text string) {
	if text == "" {
		return
	}
	m.content.WriteString(text)
	if m.ready {
		m.viewport.SetContent(m.content.String())
		if m.followTail {
			m.viewport.GotoBottom()
		}
	}
}

func (m *uiModel) renderPrompt() string {
	lines := []boxLine{
		{text: m.promptHint, style: modalBodyLineStyle()},
		{text: m.promptInput.View(), style: modalBodyLineStyle()},
		{text: "Enter submit | Esc cancel", style: modalFooterLineStyle()},
	}
	return renderBox(m.promptTitle, lines, 48, m.availableModalWidth())
}

func renderModal(title, body string, maxWidth int) string {
	if title == "" {
		title = "Info"
	}
	parts := strings.Split(strings.ReplaceAll(body, "\r\n", "\n"), "\n")
	if len(parts) > 12 {
		parts = append(parts[:12], "... (press Esc/Enter to close)")
	}
	lines := make([]boxLine, 0, len(parts))
	for _, part := range parts {
		lines = append(lines, boxLine{text: part, style: modalBodyLineStyle()})
	}
	return renderBox(title, lines, 20, maxWidth)
}

func renderPanelModal(title string, lines []panelLine, footer string, maxWidth int) string {
	boxLines := make([]boxLine, 0, len(lines)+1)
	for _, line := range lines {
		style := modalBodyLineStyle()
		prefix := "  "
		if line.selected {
			style = selectedPanelLineStyle()
			prefix = "▸ "
		}
		boxLines = append(boxLines, boxLine{text: prefix + line.text, style: style})
	}
	boxLines = append(boxLines, boxLine{text: footer, style: modalFooterLineStyle()})
	return renderBox(title, boxLines, 40, maxWidth)
}

func fillScreen(width, height int, content string) string {
	if width <= 0 || height <= 0 {
		return content
	}
	return lipgloss.Place(width, height, lipgloss.Left, lipgloss.Top, content,
		lipgloss.WithWhitespaceChars(" "),
		lipgloss.WithWhitespaceForeground(lipgloss.Color("0")),
	)
}

func renderCenteredModal(width, height int, title, body string) string {
	maxWidth := width - 8
	if maxWidth < 20 {
		maxWidth = 20
	}
	return renderCenteredModalContent(width, height, renderModal(title, body, maxWidth))
}

func renderCenteredModalContent(width, height int, content string) string {
	if width <= 0 || height <= 0 {
		return content
	}

	lines := strings.Split(content, "\n")
	blockWidth := 0
	for _, line := range lines {
		blockWidth = maxInt(blockWidth, lipgloss.Width(line))
	}
	blockHeight := len(lines)
	leftPad := 0
	if width > blockWidth {
		leftPad = (width - blockWidth) / 2
	}
	topPad := 0
	if height > blockHeight {
		topPad = (height - blockHeight) / 2
	}

	var b strings.Builder
	for i := 0; i < topPad; i++ {
		b.WriteByte('\n')
	}
	for i, line := range lines {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(strings.Repeat(" ", leftPad))
		b.WriteString(line)
	}
	return b.String()
}

func (m *uiModel) availableModalWidth() int {
	if m.width <= 0 {
		return 100
	}
	maxWidth := m.width - 8
	if maxWidth < 20 {
		maxWidth = 20
	}
	return maxWidth
}

type boxLine struct {
	text  string
	style lipgloss.Style
}

func renderBox(title string, lines []boxLine, minWidth, maxWidth int) string {
	contentWidth := lipgloss.Width(title)
	for _, line := range lines {
		contentWidth = maxInt(contentWidth, lipgloss.Width(line.text))
	}
	contentWidth = maxInt(minWidth, contentWidth)
	contentWidth = minInt(contentWidth, maxWidth)

	top := "╭" + strings.Repeat("─", contentWidth+2) + "╮"
	bottom := "╰" + strings.Repeat("─", contentWidth+2) + "╯"

	rows := make([]string, 0, len(lines)+3)
	rows = append(rows, top)
	rows = append(rows, renderBoxRow(modalHeaderLineStyle(), title, contentWidth))
	for _, line := range lines {
		rows = append(rows, renderBoxRow(line.style, truncateToWidth(line.text, contentWidth), contentWidth))
	}
	rows = append(rows, bottom)
	return strings.Join(rows, "\n")
}

func renderBoxRow(contentStyle lipgloss.Style, text string, width int) string {
	visible := truncateToWidth(text, width)
	pad := strings.Repeat(" ", maxInt(0, width-lipgloss.Width(visible)))
	inner := contentStyle.Render(visible) + pad
	return "│ " + inner + " │"
}

func modalHeaderLineStyle() lipgloss.Style {
	return lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("230")).Background(lipgloss.Color("25"))
}

func modalBodyLineStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(lipgloss.Color("252")).Background(lipgloss.Color("236"))
}

func modalFooterLineStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(lipgloss.Color("250")).Background(lipgloss.Color("236"))
}

func selectedPanelLineStyle() lipgloss.Style {
	return lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("230")).Background(lipgloss.Color("31"))
}


func truncateToWidth(s string, width int) string {
	if width <= 0 || lipgloss.Width(s) <= width {
		return s
	}
	var b strings.Builder
	for _, r := range s {
		next := b.String() + string(r)
		if lipgloss.Width(next) > width {
			break
		}
		b.WriteRune(r)
	}
	return b.String()
}

func clampIndex(idx, n int) int {
	if n <= 0 || idx < 0 {
		return 0
	}
	if idx >= n {
		return n - 1
	}
	return idx
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a int, rest ...int) int {
	max := a
	for _, v := range rest {
		if v > max {
			max = v
		}
	}
	return max
}
