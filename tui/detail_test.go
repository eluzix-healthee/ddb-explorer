package tui

import (
	"fmt"
	"strings"
	"testing"

	"ddb-explorer/aws"

	tea "github.com/charmbracelet/bubbletea"
)

func TestResultsEnterOpensItemDetailView(t *testing.T) {
	m := NewModel("dev", nil)
	m.enterResultsView("orders", aws.QueryResult{
		Items: []map[string]interface{}{
			{"account_id": "acc-1", "status": "OPEN", "notes": "short"},
		},
		RawItems: []map[string]interface{}{
			{"account_id": "acc-1", "status": "OPEN", "notes": "short"},
		},
	}, viewStateQuery)

	m = sendKey(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.state != viewStateDetail {
		t.Fatalf("expected detail state, got %q", m.state)
	}

	view := m.detailView()
	required := []string{"Item Detail", "Table: orders", "account_id:", "status:"}
	for _, value := range required {
		if !strings.Contains(view, value) {
			t.Fatalf("detail view missing %q", value)
		}
	}
}

func TestDetailJSONModalScrollsAndCloses(t *testing.T) {
	m := NewModel("dev", nil)
	m.height = 22

	tags := make([]string, 0, 32)
	for idx := 0; idx < 32; idx++ {
		tags = append(tags, fmt.Sprintf("tag-%02d", idx))
	}

	raw := map[string]interface{}{
		"account_id": "acc-1",
		"payload": map[string]interface{}{
			"status": "OPEN",
			"tags":   tags,
		},
	}
	m.enterResultsView("orders", aws.QueryResult{
		Items:    []map[string]interface{}{{"account_id": "acc-1", "payload": "{...}"}},
		RawItems: []map[string]interface{}{raw},
	}, viewStateQuery)

	m = sendKey(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	m = sendKey(t, m, tea.KeyMsg{Type: tea.KeyEnter})

	if !m.jsonModalOpen {
		t.Fatal("expected JSON modal to be open")
	}
	if m.state != viewStateDetail {
		t.Fatalf("expected detail state while modal is open, got %q", m.state)
	}

	modalView := m.View()
	if !strings.Contains(modalView, "Raw JSON Modal") {
		t.Fatalf("expected modal title in view, got %q", modalView)
	}

	m = sendKey(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if m.jsonModalScroll == 0 {
		t.Fatal("expected modal scroll offset to increase after moving down")
	}

	m = sendKey(t, m, tea.KeyMsg{Type: tea.KeyEsc})
	if m.jsonModalOpen {
		t.Fatal("expected esc to close JSON modal")
	}
	if m.state != viewStateDetail {
		t.Fatalf("expected to remain in detail view after closing modal, got %q", m.state)
	}
}

func TestJSONModalRemainsCenteredOnResize(t *testing.T) {
	m := NewModel("dev", nil)
	raw := map[string]interface{}{
		"account_id": "acc-1",
		"history": []interface{}{
			map[string]interface{}{"at": "2026-01-01", "state": "NEW"},
			map[string]interface{}{"at": "2026-01-02", "state": "OPEN"},
			map[string]interface{}{"at": "2026-01-03", "state": "CLOSED"},
		},
	}

	m.enterResultsView("orders", aws.QueryResult{
		Items:    []map[string]interface{}{{"account_id": "acc-1", "history": "[...]"}},
		RawItems: []map[string]interface{}{raw},
	}, viewStateQuery)
	m = sendKey(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	m = sendKey(t, m, tea.KeyMsg{Type: tea.KeyEnter})

	largeModel, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	large, ok := largeModel.(Model)
	if !ok {
		t.Fatalf("expected Model, got %T", largeModel)
	}
	largeView := large.View()
	if got := len(strings.Split(largeView, "\n")); got != 40 {
		t.Fatalf("expected 40 lines after large resize, got %d", got)
	}
	largeIndex := lineIndexContaining(largeView, "Raw JSON Modal")
	if largeIndex == -1 {
		t.Fatalf("missing JSON modal title in large render: %q", largeView)
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
	smallIndex := lineIndexContaining(smallView, "Raw JSON Modal")
	if smallIndex == -1 {
		t.Fatalf("missing JSON modal title in small render: %q", smallView)
	}

	if largeIndex <= smallIndex {
		t.Fatalf("expected modal to move lower on taller viewport: large=%d small=%d", largeIndex, smallIndex)
	}
}
