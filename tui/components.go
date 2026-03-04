package tui

import "github.com/charmbracelet/lipgloss"

func (m Model) renderChrome(content string) string {
	contentHeight := m.height - 2
	if contentHeight < 1 {
		contentHeight = 1
	}

	titleBar := m.styles.ChromeTitle.Width(m.width).Render(" DDB Explorer ")
	statusBar := m.styles.ChromeHint.Width(m.width).Render(m.status)
	centered := lipgloss.Place(m.width, contentHeight, lipgloss.Center, lipgloss.Center, content)

	return lipgloss.JoinVertical(lipgloss.Left, titleBar, centered, statusBar)
}

func (m Model) withHelp(content string) string {
	help := m.styles.Help.Render("q: quit • esc: back • ctrl+h: toggle help")
	return lipgloss.JoinVertical(lipgloss.Left, content, "", help)
}
