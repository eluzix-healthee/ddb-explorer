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

func (m Model) resultsView() string {
	tableName := m.activeTable
	if tableName == "" {
		tableName = "(none)"
	}

	pagination := m.resultsPaginationStatus()
	columns, hiddenColumns := m.resultsVisibleColumns()
	tableLines := m.renderResultRows(columns)
	tableContent := lipgloss.JoinVertical(lipgloss.Left, tableLines...)

	columnHint := ""
	if hiddenColumns > 0 {
		columnHint = m.theme.Hint.Render(fmt.Sprintf("Showing %d of %d columns (resize wider to reveal more).", len(columns), len(m.resultColumns)))
	}

	serverPageHint := ""
	if m.resultHasMore {
		serverPageHint = m.theme.Hint.Render("More items are available in DynamoDB beyond this local page batch.")
	}

	bodyParts := []string{
		m.theme.Title.Render("Results"),
		"",
		m.theme.Body.Render("Table: " + tableName),
		m.theme.Success.Render(pagination),
		"",
		tableContent,
	}
	if columnHint != "" {
		bodyParts = append(bodyParts, "", columnHint)
	}
	if serverPageHint != "" {
		bodyParts = append(bodyParts, serverPageHint)
	}
	bodyParts = append(bodyParts, "", m.theme.Hint.Render("Use ↑/↓ (or j/k) for rows, n/p for pages, Enter for detail view."))

	body := lipgloss.JoinVertical(lipgloss.Left, bodyParts...)
	return m.theme.Panel.Render(body)
}

func (m Model) detailView() string {
	if m.jsonModalOpen {
		return m.jsonModalView()
	}

	tableName := m.activeTable
	if tableName == "" {
		tableName = "(none)"
	}

	fields := m.selectedResultKeys()
	detailRows := m.renderDetailRows(fields)
	detailContent := lipgloss.JoinVertical(lipgloss.Left, detailRows...)

	rowLabel := "Row 0 of 0"
	if len(m.resultItems) > 0 {
		rowLabel = fmt.Sprintf("Row %d of %d", m.resultSelected+1, len(m.resultItems))
	}

	body := lipgloss.JoinVertical(
		lipgloss.Left,
		m.theme.Title.Render("Item Detail"),
		"",
		m.theme.Body.Render("Table: "+tableName),
		m.theme.Success.Render(rowLabel),
		"",
		detailContent,
		"",
		m.theme.Hint.Render("Use ↑/↓ (or j/k) to inspect fields. Press Enter to open raw JSON modal."),
	)

	return m.theme.Panel.Render(body)
}

func (m Model) jsonModalView() string {
	modalWidth := m.width - 12
	if modalWidth > 108 {
		modalWidth = 108
	}
	if modalWidth < 32 {
		modalWidth = 32
	}

	lineWidth := modalWidth - 8
	if lineWidth < 12 {
		lineWidth = 12
	}

	start := m.jsonModalScroll
	if start < 0 {
		start = 0
	}
	if start > len(m.jsonModalLines) {
		start = len(m.jsonModalLines)
	}

	end := start + m.jsonModalPageSize()
	if end > len(m.jsonModalLines) {
		end = len(m.jsonModalLines)
	}

	rendered := make([]string, 0, end-start)
	for idx := start; idx < end; idx++ {
		rendered = append(rendered, truncateRunes(m.jsonModalLines[idx], lineWidth))
	}
	if len(rendered) == 0 {
		rendered = append(rendered, "{}")
	}

	header := fmt.Sprintf("Lines %d-%d of %d", start+1, end, len(m.jsonModalLines))
	body := lipgloss.JoinVertical(
		lipgloss.Left,
		m.theme.Title.Render("Raw JSON Modal"),
		m.theme.Success.Render(header),
		"",
		strings.Join(rendered, "\n"),
		"",
		m.theme.Hint.Render("Use ↑/↓ (or j/k) to scroll lines, n/p for pages, Enter or Esc to close."),
	)

	modalPanel := m.theme.Panel
	return modalPanel.Width(modalWidth).Render(body)
}

func (m Model) resultsVisibleColumns() ([]string, int) {
	if len(m.resultColumns) == 0 {
		return nil, 0
	}

	availableWidth := m.width - 20
	if availableWidth < 16 {
		availableWidth = 16
	}

	maxColumns := availableWidth / 14
	if maxColumns < 1 {
		maxColumns = 1
	}
	if maxColumns > len(m.resultColumns) {
		maxColumns = len(m.resultColumns)
	}

	visible := make([]string, maxColumns)
	copy(visible, m.resultColumns[:maxColumns])

	return visible, len(m.resultColumns) - len(visible)
}

func (m Model) resultColumnWidth(columnCount int) int {
	if columnCount <= 0 {
		return 12
	}

	availableWidth := m.width - 20
	if availableWidth < 16 {
		availableWidth = 16
	}

	separatorWidth := (columnCount - 1) * 3
	columnWidth := (availableWidth - separatorWidth) / columnCount
	if columnWidth < 8 {
		return 8
	}
	if columnWidth > 28 {
		return 28
	}

	return columnWidth
}

func (m Model) renderResultRows(columns []string) []string {
	if len(m.resultItems) == 0 {
		return []string{m.theme.Hint.Render("No items returned.")}
	}
	if len(columns) == 0 {
		return []string{m.theme.Hint.Render("No columns discovered in result rows.")}
	}

	columnWidth := m.resultColumnWidth(len(columns))
	formatCells := func(values []string) string {
		cells := make([]string, len(values))
		for idx, value := range values {
			cells[idx] = fmt.Sprintf("%-*s", columnWidth, truncateRunes(value, columnWidth))
		}
		return strings.Join(cells, " | ")
	}

	headerValues := make([]string, len(columns))
	copy(headerValues, columns)
	rows := []string{m.theme.Highlight.Render("  " + formatCells(headerValues))}

	start, end := m.currentResultPageRange()
	for idx := start; idx < end; idx++ {
		item := m.resultItems[idx]
		values := make([]string, 0, len(columns))
		for _, column := range columns {
			values = append(values, fmt.Sprint(item[column]))
		}

		row := "  " + formatCells(values)
		if idx == m.resultSelected {
			rows = append(rows, m.theme.Highlight.Render("> "+formatCells(values)))
			continue
		}

		rows = append(rows, row)
	}

	return rows
}

func (m Model) renderDetailRows(fields []string) []string {
	if len(fields) == 0 {
		return []string{m.theme.Hint.Render("No fields available for this item.")}
	}

	item, _, ok := m.selectedResultItems()
	if !ok {
		return []string{m.theme.Hint.Render("No fields available for this item.")}
	}

	start := m.detailScroll
	if start < 0 {
		start = 0
	}
	if start >= len(fields) {
		start = len(fields) - 1
	}
	if start < 0 {
		start = 0
	}

	pageSize := m.detailPageSize()
	end := start + pageSize
	if end > len(fields) {
		end = len(fields)
	}

	labelWidth := 24
	valueWidth := m.width - labelWidth - 30
	if valueWidth < 14 {
		valueWidth = 14
	}

	lines := make([]string, 0, (end-start)*3+1)
	for idx := start; idx < end; idx++ {
		field := fields[idx]
		value := fmt.Sprint(item[field])
		wrapped := wrapRunes(value, valueWidth)
		if len(wrapped) == 0 {
			wrapped = []string{""}
		}

		marker := "  "
		if idx == m.detailSelected {
			marker = "> "
		}

		head := fmt.Sprintf("%s%-*s %s", marker, labelWidth, field+":", wrapped[0])
		if idx == m.detailSelected {
			head = m.theme.Highlight.Render(head)
		}
		lines = append(lines, head)

		maxContinuation := 2
		for lineIdx := 1; lineIdx < len(wrapped) && lineIdx <= maxContinuation; lineIdx++ {
			lines = append(lines, fmt.Sprintf("  %-*s %s", labelWidth, "", wrapped[lineIdx]))
		}
		if len(wrapped) > maxContinuation+1 {
			lines = append(lines, m.theme.Hint.Render(fmt.Sprintf("  %-*s %s", labelWidth, "", "…")))
		}
	}

	if len(fields) > pageSize {
		lines = append(lines, m.theme.Hint.Render(fmt.Sprintf("Showing fields %d-%d of %d", start+1, end, len(fields))))
	}

	return lines
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

func wrapRunes(value string, limit int) []string {
	if limit <= 0 {
		return []string{""}
	}

	normalized := strings.ReplaceAll(value, "\r\n", "\n")
	segments := strings.Split(normalized, "\n")
	wrapped := make([]string, 0, len(segments))
	for _, segment := range segments {
		runes := []rune(segment)
		if len(runes) == 0 {
			wrapped = append(wrapped, "")
			continue
		}

		for len(runes) > limit {
			wrapped = append(wrapped, string(runes[:limit]))
			runes = runes[limit:]
		}
		wrapped = append(wrapped, string(runes))
	}

	if len(wrapped) == 0 {
		return []string{""}
	}

	return wrapped
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
