package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestInvalidTransitionFallsBackToErrorState(t *testing.T) {
	m := loadTablesForTest(t)
	err := m.transitionTo(viewStateResults)
	if err == nil {
		t.Fatal("expected transition validation to fail")
	}
	m.transitionFailure(err)

	if m.state != viewStateError {
		t.Fatalf("expected error state after invalid transition, got %q", m.state)
	}
	if m.err == nil {
		t.Fatal("expected transition error details")
	}
	if !strings.Contains(m.err.Error(), "tables") || !strings.Contains(m.err.Error(), "results") {
		t.Fatalf("expected transition error to include source/target states, got %q", m.err.Error())
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

func TestHelpToggleOpensShortcutsOverlay(t *testing.T) {
	m := loadTablesForTest(t)
	m.width = 120
	m.height = 24

	compactView := m.View()
	if !strings.Contains(compactView, "Press Ctrl+H for shortcuts") {
		t.Fatalf("expected compact help prompt, got %q", compactView)
	}

	m = sendKey(t, m, tea.KeyMsg{Type: tea.KeyCtrlH})
	overlayView := m.View()
	required := []string{"Keyboard Shortcuts", "q: quit", "esc: back", "ctrl+h: toggle help", "/: filter", "enter: open query/scan"}
	for _, token := range required {
		if !strings.Contains(overlayView, token) {
			t.Fatalf("expected help overlay to include %q", token)
		}
	}

	m = sendKey(t, m, tea.KeyMsg{Type: tea.KeyEsc})
	closedView := m.View()
	if !strings.Contains(closedView, "Press Ctrl+H for shortcuts") {
		t.Fatalf("expected compact help prompt after closing overlay, got %q", closedView)
	}
}

func TestQueryHelpIncludesSortConditionControl(t *testing.T) {
	m := loadTablesForTest(t)
	m.width = 160
	m.height = 24
	m = sendKey(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	m = sendKey(t, m, tea.KeyMsg{Type: tea.KeyCtrlH})

	view := m.View()
	required := []string{"[ / ]: sort cond", "enter: run query", "ctrl+s: switch to scan"}
	for _, token := range required {
		if !strings.Contains(view, token) {
			t.Fatalf("expected query help to include %q", token)
		}
	}
}
