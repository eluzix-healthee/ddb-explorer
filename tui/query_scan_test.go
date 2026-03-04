package tui

import (
	"strings"
	"testing"

	"ddb-explorer/aws"

	tea "github.com/charmbracelet/bubbletea"
)

func TestQueryFormSupportsRequiredAndOptionalFields(t *testing.T) {
	m := NewModel("dev", nil)
	m.tables = []aws.TableInfo{{
		Name:         "orders",
		PartitionKey: "account_id",
		SortKey:      "created_at",
	}}
	m.state = viewStateTables

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	opened, ok := next.(Model)
	if !ok {
		t.Fatalf("expected Model, got %T", next)
	}

	if opened.state != viewStateQuery {
		t.Fatalf("expected query view, got %q", opened.state)
	}
	if got := opened.queryFields[queryFieldPartitionKey].value; got != "account_id" {
		t.Fatalf("expected partition key to preload from table schema, got %q", got)
	}
	if got := opened.queryFields[queryFieldSortKey].value; got != "created_at" {
		t.Fatalf("expected sort key to preload from table schema, got %q", got)
	}

	view := opened.queryView()
	required := []string{"Partition Key Name", "Partition Key Value", "Sort Key Name", "Sort Key Value", "Index Name", "Run Query"}
	for _, value := range required {
		if !strings.Contains(view, value) {
			t.Fatalf("query view missing %q", value)
		}
	}
}

func TestQueryFormFocusOrderAndEditing(t *testing.T) {
	m := loadTablesForTest(t)
	m = sendKey(t, m, tea.KeyMsg{Type: tea.KeyEnter})

	if m.queryFocus != 0 {
		t.Fatalf("expected initial focus on first field, got %d", m.queryFocus)
	}

	m = sendKey(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("pk")})
	if got := m.queryFields[queryFieldPartitionKey].value; got != "pk" {
		t.Fatalf("expected partition key input edit, got %q", got)
	}

	m = sendKey(t, m, tea.KeyMsg{Type: tea.KeyTab})
	if m.queryFocus != 1 {
		t.Fatalf("expected focus on second field after tab, got %d", m.queryFocus)
	}

	m = sendKey(t, m, tea.KeyMsg{Type: tea.KeyShiftTab})
	if m.queryFocus != 0 {
		t.Fatalf("expected focus to move back with shift+tab, got %d", m.queryFocus)
	}
}

func TestQueryRunRequiresExplicitActionFocus(t *testing.T) {
	m := loadTablesForTest(t)
	m = sendKey(t, m, tea.KeyMsg{Type: tea.KeyEnter})

	m.queryFields[queryFieldPartitionKey].value = "user_id"
	m.queryFields[queryFieldPartitionValue].value = "123"
	m.queryFocus = len(m.queryFields)

	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	updated, ok := next.(Model)
	if !ok {
		t.Fatalf("expected Model, got %T", next)
	}
	if cmd == nil {
		t.Fatal("expected query run command when activating Run Query")
	}
	if !strings.Contains(updated.status, "running query") {
		t.Fatalf("expected running query status, got %q", updated.status)
	}
}

func TestScanRunUsesExplicitTrigger(t *testing.T) {
	m := loadTablesForTest(t)
	m = sendKey(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	m = sendKey(t, m, tea.KeyMsg{Type: tea.KeyCtrlS})

	if m.state != viewStateScan {
		t.Fatalf("expected scan view after ctrl+s, got %q", m.state)
	}

	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	updated, ok := next.(Model)
	if !ok {
		t.Fatalf("expected Model, got %T", next)
	}
	if cmd == nil {
		t.Fatal("expected scan command when pressing enter in scan view")
	}
	if !strings.Contains(updated.status, "running scan") {
		t.Fatalf("expected running scan status, got %q", updated.status)
	}
}

func TestQueryFormRemainsCenteredOnResize(t *testing.T) {
	m := loadTablesForTest(t)
	m = sendKey(t, m, tea.KeyMsg{Type: tea.KeyEnter})

	largeModel, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	large, ok := largeModel.(Model)
	if !ok {
		t.Fatalf("expected Model, got %T", largeModel)
	}
	largeView := large.View()
	if got := len(strings.Split(largeView, "\n")); got != 40 {
		t.Fatalf("expected 40 lines after large resize, got %d", got)
	}
	largeIndex := lineIndexContaining(largeView, "Partition Key Name")
	if largeIndex == -1 {
		t.Fatalf("missing query form row in large view: %q", largeView)
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
	smallIndex := lineIndexContaining(smallView, "Partition Key Name")
	if smallIndex == -1 {
		t.Fatalf("missing query form row in small view: %q", smallView)
	}

	if largeIndex <= smallIndex {
		t.Fatalf("expected query form row to move lower on taller viewport: large=%d small=%d", largeIndex, smallIndex)
	}
}
