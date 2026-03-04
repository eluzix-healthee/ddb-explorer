package tui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

func (m Model) loadingView() string {
	body := lipgloss.JoinVertical(
		lipgloss.Center,
		m.styles.Title.Render("DDB Explorer"),
		"",
		m.styles.Highlight.Render(m.spinner.View()+" Connecting to DynamoDB..."),
		"",
		m.styles.Hint.Render("Press q to quit"),
	)
	return m.styles.Panel.Render(body)
}

func (m Model) tablesView() string {
	body := lipgloss.JoinVertical(
		lipgloss.Left,
		m.styles.Title.Render("DDB Explorer"),
		"",
		m.styles.Success.Render("Bubble Tea app skeleton is running."),
		m.styles.Body.Render("Table list/query views will be added in upcoming stories."),
		"",
		m.styles.Hint.Render("Global controls are already wired."),
	)
	return m.styles.Panel.Render(body)
}

func (m Model) errorView() string {
	errText := "unknown error"
	if m.err != nil {
		errText = m.err.Error()
	}

	body := lipgloss.JoinVertical(
		lipgloss.Left,
		m.styles.Title.Render("DDB Explorer"),
		"",
		m.styles.Error.Render("Unable to connect to DynamoDB."),
		m.styles.Body.Render(fmt.Sprintf("Details: %s", errText)),
		"",
		m.styles.Hint.Render("Verify AWS credentials/profile and relaunch."),
	)
	return m.styles.Panel.Render(body)
}
