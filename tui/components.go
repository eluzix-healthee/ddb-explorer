package tui

import "github.com/charmbracelet/lipgloss"

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
	centered := lipgloss.Place(renderWidth, contentHeight, lipgloss.Center, lipgloss.Center, content)

	return lipgloss.JoinVertical(lipgloss.Left, titleBar, centered, helpBar, statusBar)
}

func (m Model) helpBarText(width int) string {
	if !m.showHelp {
		return "Press Ctrl+H for shortcuts"
	}

	helpText := m.keys.HelpText(m.state)
	if helpText == "" {
		helpText = "No shortcuts available"
	}

	return truncateRunes(helpText, maxChromeTextWidth(width))
}

func maxChromeTextWidth(width int) int {
	if width <= 2 {
		return 1
	}

	return width - 2
}
