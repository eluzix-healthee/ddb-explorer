package tui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

func (m Model) loadingView() string {
	profileText := m.profile
	if profileText == "" {
		profileText = "default"
	}

	body := lipgloss.JoinVertical(
		lipgloss.Center,
		m.theme.Title.Render("DDB Explorer"),
		"",
		m.theme.Highlight.Render(m.spinner.View()+" Connecting to DynamoDB"),
		m.theme.Body.Render("Loading table metadata..."),
		m.theme.Hint.Render("Profile: "+profileText),
		"",
		m.theme.Hint.Render("Press q to quit"),
	)
	return m.theme.Panel.Render(body)
}

func (m Model) tablesView() string {
	statusText := fmt.Sprintf("Loaded %d tables.", len(m.tables))
	if len(m.tables) == 0 {
		statusText = "No tables discovered for this profile."
	}

	body := lipgloss.JoinVertical(
		lipgloss.Left,
		m.theme.Title.Render("DDB Explorer"),
		"",
		m.theme.Success.Render(statusText),
		m.theme.Body.Render("Table list/query views will be added in upcoming stories."),
		"",
		m.theme.Hint.Render("Press ctrl+h to see global and table-list key bindings."),
	)
	return m.theme.Panel.Render(body)
}

func (m Model) errorView() string {
	errText := "unknown error"
	if m.err != nil {
		errText = m.err.Error()
	}

	body := lipgloss.JoinVertical(
		lipgloss.Left,
		m.theme.Title.Render("DDB Explorer"),
		"",
		m.theme.Error.Render("Unable to connect to DynamoDB."),
		m.theme.Body.Render(fmt.Sprintf("Details: %s", errText)),
		"",
		m.theme.Hint.Render("Verify AWS credentials/profile and relaunch."),
	)
	return m.theme.Panel.Render(body)
}
