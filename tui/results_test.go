package tui

import (
	"fmt"
	"strings"
	"testing"

	"ddb-explorer/aws"

	tea "github.com/charmbracelet/bubbletea"
)

func TestQuerySuccessTransitionsToResultsWithAdaptiveColumns(t *testing.T) {
	m := loadTablesForTest(t)
	m = sendKey(t, m, tea.KeyMsg{Type: tea.KeyEnter})

	result := aws.QueryResult{
		Items: []map[string]interface{}{
			{"account_id": "a1", "status": "OPEN", "created_at": "2026-03-04"},
			{"account_id": "a2", "amount": "120", "status": "PAID"},
		},
		LastEvaluatedKey: map[string]interface{}{"account_id": "a2"},
	}
	m.pendingQueryRequestID = 42

	next, _ := m.Update(querySuccessMsg{requestID: 42, tableName: "orders", result: result})
	updated, ok := next.(Model)
	if !ok {
		t.Fatalf("expected Model, got %T", next)
	}

	if updated.state != viewStateResults {
		t.Fatalf("expected results state, got %q", updated.state)
	}
	if updated.resultOrigin != viewStateQuery {
		t.Fatalf("expected results origin query, got %q", updated.resultOrigin)
	}
	if len(updated.resultColumns) != 4 {
		t.Fatalf("expected 4 discovered columns, got %d", len(updated.resultColumns))
	}

	view := updated.resultsView()
	required := []string{"Results", "Table: orders", "Page 1/1", "account_id", "status"}
	for _, item := range required {
		if !strings.Contains(view, item) {
			t.Fatalf("results view missing %q", item)
		}
	}
}

func TestResultsKeyboardNavigationMovesRowsAndPages(t *testing.T) {
	m := NewModel("dev", nil)
	m.height = 22

	items := make([]map[string]interface{}, 0, 12)
	for idx := 1; idx <= 12; idx++ {
		items = append(items, map[string]interface{}{
			"id":    fmt.Sprintf("item-%02d", idx),
			"state": "ok",
		})
	}

	if err := m.enterResultsView("orders", aws.QueryResult{Items: items}, viewStateQuery); err != nil {
		t.Fatalf("expected results view entry, got error: %v", err)
	}
	m = sendKey(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	m = sendKey(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if m.resultSelected != 2 {
		t.Fatalf("expected selected row 2 after moving down twice, got %d", m.resultSelected)
	}
	if m.resultPage != 0 {
		t.Fatalf("expected to remain on page 0, got %d", m.resultPage)
	}

	m = sendKey(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	if m.resultPage != 1 {
		t.Fatalf("expected page 1 after next-page key, got %d", m.resultPage)
	}
	if m.resultSelected != 10 {
		t.Fatalf("expected selection to preserve in-page offset on next page, got row %d", m.resultSelected)
	}

	m = sendKey(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
	if m.resultPage != 0 {
		t.Fatalf("expected page 0 after previous-page key, got %d", m.resultPage)
	}
}

func TestResultsViewRemainsCenteredOnResize(t *testing.T) {
	m := NewModel("dev", nil)
	items := []map[string]interface{}{
		{"id": "item-01", "state": "OPEN"},
		{"id": "item-02", "state": "CLOSED"},
	}
	if err := m.enterResultsView("orders", aws.QueryResult{Items: items}, viewStateQuery); err != nil {
		t.Fatalf("expected results view entry, got error: %v", err)
	}

	largeModel, _ := m.Update(tea.WindowSizeMsg{Width: 110, Height: 36})
	large, ok := largeModel.(Model)
	if !ok {
		t.Fatalf("expected Model, got %T", largeModel)
	}
	largeView := large.View()
	if got := len(strings.Split(largeView, "\n")); got != 36 {
		t.Fatalf("expected 36 lines after large resize, got %d", got)
	}
	largeIndex := lineIndexContaining(largeView, "Results")
	if largeIndex == -1 {
		t.Fatalf("missing results title in large render: %q", largeView)
	}

	smallModel, _ := large.Update(tea.WindowSizeMsg{Width: 80, Height: 20})
	small, ok := smallModel.(Model)
	if !ok {
		t.Fatalf("expected Model, got %T", smallModel)
	}
	smallView := small.View()
	if got := len(strings.Split(smallView, "\n")); got != 20 {
		t.Fatalf("expected 20 lines after small resize, got %d", got)
	}
	smallIndex := lineIndexContaining(smallView, "Results")
	if smallIndex == -1 {
		t.Fatalf("missing results title in small render: %q", smallView)
	}

	if largeIndex != smallIndex {
		t.Fatalf("expected stretched results content to stay top-aligned across sizes: large=%d small=%d", largeIndex, smallIndex)
	}
}

func TestResultsColumnsExcludeAnyGSIPrefixedField(t *testing.T) {
	m := NewModel("dev", nil)
	m.state = viewStateResults

	result := aws.QueryResult{Items: []map[string]interface{}{{
		"gsi_pk":        "active",
		"GSI_secondary": "secondary",
		"gsi_sk":        "2026-03-04",
		"pk":            "tenant-1",
		"sk":            "contract-1",
		"status":        "OPEN",
		"name":          "Sample",
	}}}
	columns := m.discoverResultColumnsForOrigin(result.Items, viewStateScan)

	for _, column := range columns {
		if strings.HasPrefix(strings.ToUpper(column), "GSI") {
			t.Fatalf("expected GSI-prefixed fields to be filtered out, got columns %v", columns)
		}
	}
	if !containsString(columns, "status") {
		t.Fatalf("expected non-GSI columns to remain, got %v", columns)
	}
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}

	return false
}
