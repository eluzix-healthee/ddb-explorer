package tui

import (
	"strings"
	"testing"

	"ddb-explorer/aws"

	tea "github.com/charmbracelet/bubbletea"
)

func TestInvalidTransitionFallsBackToErrorState(t *testing.T) {
	m := loadTablesForTest(t)

	next, _ := m.Update(querySuccessMsg{tableName: "orders", result: aws.QueryResult{}})
	updated, ok := next.(Model)
	if !ok {
		t.Fatalf("expected Model, got %T", next)
	}

	if updated.state != viewStateError {
		t.Fatalf("expected error state after invalid transition, got %q", updated.state)
	}
	if updated.err == nil {
		t.Fatal("expected transition error details")
	}
	if !strings.Contains(updated.err.Error(), "tables") || !strings.Contains(updated.err.Error(), "results") {
		t.Fatalf("expected transition error to include source/target states, got %q", updated.err.Error())
	}
}

func TestChromeRendersHelpAndStatusAcrossViews(t *testing.T) {
	states := []viewState{
		viewStateLoading,
		viewStateTables,
		viewStateQuery,
		viewStateScan,
		viewStateResults,
		viewStateDetail,
		viewStateError,
	}

	for _, state := range states {
		m := NewModel("dev", nil)
		m.width = 96
		m.height = 26
		m.state = state
		m.status = "status for " + string(state)

		view := m.View()
		if got := len(strings.Split(view, "\n")); got != m.height {
			t.Fatalf("state %q expected %d rendered lines, got %d", state, m.height, got)
		}
		if !strings.Contains(view, "DDB Explorer") {
			t.Fatalf("state %q missing title chrome", state)
		}
		if !strings.Contains(view, m.status) {
			t.Fatalf("state %q missing status chrome", state)
		}
		if !strings.Contains(view, "Press Ctrl+H for shortcuts") {
			t.Fatalf("state %q missing compact help chrome", state)
		}
	}
}

func TestHelpToggleExpandsStateSpecificHelpInChrome(t *testing.T) {
	m := loadTablesForTest(t)
	m.width = 120
	m.height = 24

	compactView := m.View()
	if !strings.Contains(compactView, "Press Ctrl+H for shortcuts") {
		t.Fatalf("expected compact help prompt, got %q", compactView)
	}

	m = sendKey(t, m, tea.KeyMsg{Type: tea.KeyCtrlH})
	expandedView := m.View()
	required := []string{"q: quit", "esc: back", "ctrl+h: toggle help", "/: filter", "enter: open query/scan"}
	for _, token := range required {
		if !strings.Contains(expandedView, token) {
			t.Fatalf("expected expanded help to include %q", token)
		}
	}
}
