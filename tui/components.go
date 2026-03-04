package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func (m Model) renderChrome(content string) string {
	renderWidth := m.width
	if renderWidth < 1 {
		renderWidth = 1
	}

	renderHeight := m.height
	if renderHeight < 4 {
		renderHeight = 4
	}

	contentHeight := renderHeight - 3
	if contentHeight < 1 {
		contentHeight = 1
	}

	titleBar := m.theme.ChromeTitle.Width(renderWidth).Render(" DDB Explorer ")
	helpBar := m.theme.ChromeHint.Width(renderWidth).Render(m.helpBarText(renderWidth))
	statusBar := m.theme.ChromeHint.Width(renderWidth).Render(truncateRunes(m.status, maxChromeTextWidth(renderWidth)))
	contentFrame := lipgloss.NewStyle().
		MaxWidth(renderWidth).
		MaxHeight(contentHeight).
		Render(content)
	centered := lipgloss.Place(renderWidth, contentHeight, lipgloss.Center, lipgloss.Center, contentFrame)

	return lipgloss.JoinVertical(lipgloss.Left, titleBar, centered, helpBar, statusBar)
}

func (m Model) helpBarText(width int) string {
	if m.showHelp {
		return truncateRunes("Help overlay open (Ctrl+H or Esc to close)", maxChromeTextWidth(width))
	}

	return truncateRunes("Press Ctrl+H for shortcuts", maxChromeTextWidth(width))
}

func (m Model) helpOverlayView() string {
	lines := m.keys.HelpLines(m.state)
	if len(lines) == 0 {
		lines = []string{"No shortcuts available for this view."}
	}

	renderLines := make([]string, 0, len(lines)+3)
	renderLines = append(renderLines, m.theme.Title.Render("Keyboard Shortcuts"), "")
	for _, line := range lines {
		renderLines = append(renderLines, "  "+line)
	}
	renderLines = append(renderLines, "", m.theme.Hint.Render("Press Esc or Ctrl+H to close."))

	body := lipgloss.JoinVertical(lipgloss.Left, renderLines...)
	panelWidth := m.width - 12
	if panelWidth < 44 {
		panelWidth = 44
	}

	contentHeight := m.chromeContentHeight()
	panel := m.theme.Panel.Width(panelWidth).Render(body)
	return lipgloss.Place(m.width, contentHeight, lipgloss.Center, lipgloss.Center, strings.TrimRight(panel, "\n"))
}

func maxChromeTextWidth(width int) int {
	if width <= 2 {
		return 1
	}

	return width - 2
}
