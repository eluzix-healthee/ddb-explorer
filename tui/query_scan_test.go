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
	if got := opened.currentQueryTargetLabel(); got != "Table [account_id, created_at]" {
		t.Fatalf("expected derived table key label, got %q", got)
	}

	view := opened.queryView()
	required := []string{"Source:", "account_id Value", "created_at Condition", "created_at Value", "Run Query"}
	for _, value := range required {
		if !strings.Contains(view, value) {
			t.Fatalf("query view missing %q", value)
		}
	}
}

func TestQuerySourceCyclesThroughGSI(t *testing.T) {
	m := NewModel("dev", nil)
	m.tables = []aws.TableInfo{{
		Name:         "orders",
		PartitionKey: "account_id",
		SortKey:      "created_at",
		GSIKeys: []aws.IndexKeyInfo{{
			Name:         "gsi_status",
			PartitionKey: "status",
			SortKey:      "updated_at",
		}},
	}}
	m.state = viewStateTables
	m = sendKey(t, m, tea.KeyMsg{Type: tea.KeyEnter})

	if got := m.currentQueryTargetLabel(); got != "Table [account_id, created_at]" {
		t.Fatalf("expected table source by default, got %q", got)
	}

	m = sendKey(t, m, tea.KeyMsg{Type: tea.KeyCtrlG})
	if got := m.currentQueryTargetLabel(); got != "GSI: gsi_status [status, updated_at]" {
		t.Fatalf("expected gsi source after ctrl+g, got %q", got)
	}
}

func TestQuerySortConditionCyclesWithOptionKeys(t *testing.T) {
	m := loadTablesForTest(t)
	m = sendKey(t, m, tea.KeyMsg{Type: tea.KeyEnter})

	m.queryFocus = queryFieldSortCondition
	if got := m.queryFields[queryFieldSortCondition].value; got != "=" {
		t.Fatalf("expected default sort condition '=', got %q", got)
	}

	m = sendKey(t, m, tea.KeyMsg{Type: tea.KeyRight})
	if got := m.queryFields[queryFieldSortCondition].value; got != "begins_with" {
		t.Fatalf("expected next sort condition 'begins_with', got %q", got)
	}

	m = sendKey(t, m, tea.KeyMsg{Type: tea.KeyLeft})
	if got := m.queryFields[queryFieldSortCondition].value; got != "=" {
		t.Fatalf("expected previous sort condition '=', got %q", got)
	}
}

func TestQueryFormFocusOrderAndEditing(t *testing.T) {
	m := loadTablesForTest(t)
	m = sendKey(t, m, tea.KeyMsg{Type: tea.KeyEnter})

	if m.queryFocus != 0 {
		t.Fatalf("expected initial focus on first field, got %d", m.queryFocus)
	}

	m = sendKey(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("pk")})
	if got := m.queryFields[queryFieldPartitionValue].value; got != "pk" {
		t.Fatalf("expected partition key value input edit, got %q", got)
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

	m.queryFields[queryFieldPartitionValue].value = "123"
	m.queryFocus = m.queryInputCount()

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

func TestBetweenConditionShowsEndValueInput(t *testing.T) {
	m := loadTablesForTest(t)
	m = sendKey(t, m, tea.KeyMsg{Type: tea.KeyEnter})

	if got := m.queryInputCount(); got != 3 {
		t.Fatalf("expected 3 query inputs for default condition, got %d", got)
	}

	m.queryFocus = queryFieldSortCondition
	for m.queryFields[queryFieldSortCondition].value != "between" {
		m = sendKey(t, m, tea.KeyMsg{Type: tea.KeyRight})
	}

	if got := m.queryInputCount(); got != 4 {
		t.Fatalf("expected 4 query inputs for between condition, got %d", got)
	}

	view := m.queryView()
	if !strings.Contains(view, "Value (End)") {
		t.Fatalf("expected between condition to render end value input, got %q", view)
	}
}

func TestBuildQueryRequestRequiresBetweenEndValue(t *testing.T) {
	m := loadTablesForTest(t)
	m = sendKey(t, m, tea.KeyMsg{Type: tea.KeyEnter})

	m.queryFields[queryFieldPartitionValue].value = "acc-1"
	m.queryFields[queryFieldSortCondition].value = "between"
	m.queryFields[queryFieldSortValue].value = "2026-01-01"
	m.queryFields[queryFieldSortValueEnd].value = ""

	_, err := m.buildQueryRequest()
	if err == nil {
		t.Fatal("expected buildQueryRequest to fail when between end value is missing")
	}
	if !strings.Contains(err.Error(), "end value") {
		t.Fatalf("expected between validation error, got %v", err)
	}

	m.queryFields[queryFieldSortValueEnd].value = "2026-01-31"
	request, err := m.buildQueryRequest()
	if err != nil {
		t.Fatalf("expected valid between request, got %v", err)
	}
	if request.condition != "between" {
		t.Fatalf("expected between condition, got %q", request.condition)
	}
	if request.sortValue != "2026-01-01" || request.sortValueEnd != "2026-01-31" {
		t.Fatalf("expected both between bounds, got start=%q end=%q", request.sortValue, request.sortValueEnd)
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
	largeIndex := lineIndexContaining(largeView, "Source:")
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
	smallIndex := lineIndexContaining(smallView, "Source:")
	if smallIndex == -1 {
		t.Fatalf("missing query form row in small view: %q", smallView)
	}

	if largeIndex <= smallIndex {
		t.Fatalf("expected query form row to move lower on taller viewport: large=%d small=%d", largeIndex, smallIndex)
	}
}
