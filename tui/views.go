package tui

import (
	"ddb-explorer/aws"
	"fmt"
	"strings"

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
	visibleTables := m.filteredTables()
	statusText := fmt.Sprintf("Loaded %d tables (%d visible).", len(m.tables), len(visibleTables))
	if len(m.tables) == 0 {
		statusText = "No tables discovered for this profile."
	}

	filterLabel := "Filter: (none)"
	if strings.TrimSpace(m.tableFilter) != "" {
		filterLabel = fmt.Sprintf("Filter: %q", m.tableFilter)
	}
	if m.filterInputActive {
		filterLabel = filterLabel + " [editing]"
	}

	header := fmt.Sprintf("%-34s %10s %10s %-12s", "Name", "Items", "Size", "Status")
	rows := m.renderTableRows(visibleTables)
	tableLines := make([]string, 0, len(rows)+1)
	tableLines = append(tableLines, m.theme.Highlight.Render(header))
	tableLines = append(tableLines, rows...)
	tableContent := lipgloss.JoinVertical(lipgloss.Left, tableLines...)

	body := lipgloss.JoinVertical(
		lipgloss.Left,
		m.theme.Title.Render("DDB Explorer"),
		"",
		m.theme.Success.Render(statusText),
		m.theme.Body.Render(filterLabel),
		"",
		tableContent,
		"",
		m.theme.Hint.Render("Use / to edit filter, ↑/↓ or j/k to move, enter to open query/scan flow."),
	)
	return m.theme.Panel.Render(body)
}

func (m Model) queryView() string {
	tableName := m.activeTable
	if tableName == "" {
		tableName = "(none)"
	}

	formLines := m.renderQueryFields()
	formContent := lipgloss.JoinVertical(lipgloss.Left, formLines...)

	body := lipgloss.JoinVertical(
		lipgloss.Left,
		m.theme.Title.Render("Query Flow"),
		"",
		m.theme.Body.Render("Table: "+tableName),
		"",
		formContent,
		"",
		m.theme.Hint.Render("Tab/Shift+Tab to move focus, Enter to activate Run Query, Ctrl+S to switch to scan."),
	)
	return m.theme.Panel.Render(body)
}

func (m Model) scanView() string {
	tableName := m.activeTable
	if tableName == "" {
		tableName = "(none)"
	}

	body := lipgloss.JoinVertical(
		lipgloss.Left,
		m.theme.Title.Render("Scan Flow"),
		"",
		m.theme.Body.Render("Table: "+tableName),
		m.theme.Body.Render("Scan uses the selected table and runs only when explicitly triggered."),
		"",
		m.renderScanAction(),
		"",
		m.theme.Hint.Render("Press Enter to run scan, or Esc to return to tables."),
	)
	return m.theme.Panel.Render(body)
}

func (m Model) renderQueryFields() []string {
	lines := make([]string, 0, len(m.queryFields)+2)
	for idx, field := range m.queryFields {
		marker := "  "
		if idx == m.queryFocus {
			marker = "> "
		}

		label := field.label
		if field.required {
			label += " *"
		}

		value := field.value
		if strings.TrimSpace(value) == "" {
			value = m.theme.Hint.Render(field.placeholder)
		}

		row := fmt.Sprintf("%s%-24s %s", marker, label+":", value)
		if idx == m.queryFocus {
			row = m.theme.Highlight.Render(row)
		}

		lines = append(lines, row)
	}

	action := "  [ Run Query ]"
	if m.queryFocus == len(m.queryFields) {
		action = m.theme.Highlight.Render("> [ Run Query ]")
	}
	lines = append(lines, "")
	lines = append(lines, action)

	return lines
}

func (m Model) renderScanAction() string {
	return m.theme.Highlight.Render("> [ Run Scan ]")
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

func (m Model) renderTableRows(tables []aws.TableInfo) []string {
	if len(tables) == 0 {
		return []string{m.theme.Hint.Render("No tables match the current filter.")}
	}

	selected := m.selectedTable
	if selected < 0 {
		selected = 0
	}
	if selected >= len(tables) {
		selected = len(tables) - 1
	}

	maxRows := m.height - 16
	if maxRows < 4 {
		maxRows = 4
	}

	start := 0
	if selected >= maxRows {
		start = selected - maxRows + 1
	}
	if start+maxRows > len(tables) {
		start = len(tables) - maxRows
		if start < 0 {
			start = 0
		}
	}
	end := start + maxRows
	if end > len(tables) {
		end = len(tables)
	}

	rows := make([]string, 0, (end-start)+1)
	for idx := start; idx < end; idx++ {
		table := tables[idx]
		row := fmt.Sprintf(
			"%-34s %10d %10s %-12s",
			truncateRunes(table.Name, 34),
			table.ItemCount,
			formatBytes(table.SizeBytes),
			truncateRunes(table.Status, 12),
		)
		if idx == selected {
			rows = append(rows, m.theme.Highlight.Render("> "+row))
			continue
		}
		rows = append(rows, "  "+row)
	}

	if len(tables) > maxRows {
		rows = append(rows, m.theme.Hint.Render(fmt.Sprintf("Showing %d-%d of %d", start+1, end, len(tables))))
	}

	return rows
}

func truncateRunes(value string, limit int) string {
	if limit <= 0 {
		return ""
	}

	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	if limit == 1 {
		return "…"
	}

	return string(runes[:limit-1]) + "…"
}

func formatBytes(sizeBytes int64) string {
	const (
		unit       = int64(1024)
		kiloSymbol = "KB"
		megaSymbol = "MB"
		gigaSymbol = "GB"
	)

	if sizeBytes < unit {
		return fmt.Sprintf("%dB", sizeBytes)
	}

	if sizeBytes < unit*unit {
		return fmt.Sprintf("%.1f%s", float64(sizeBytes)/float64(unit), kiloSymbol)
	}
	if sizeBytes < unit*unit*unit {
		return fmt.Sprintf("%.1f%s", float64(sizeBytes)/float64(unit*unit), megaSymbol)
	}

	return fmt.Sprintf("%.1f%s", float64(sizeBytes)/float64(unit*unit*unit), gigaSymbol)
}
