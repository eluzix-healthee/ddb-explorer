package tui

import (
	"strings"
	"testing"

	"ddb-explorer/aws"

	tea "github.com/charmbracelet/bubbletea"
)

func TestTableFilterUpdatesVisibleRows(t *testing.T) {
	m := loadTablesForTest(t)

	m = sendKey(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	if !m.filterInputActive {
		t.Fatal("expected filter input to be active")
	}

	m = sendKey(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'o'}})
	if got := len(m.filteredTables()); got != 2 {
		t.Fatalf("expected 2 tables after first filter character, got %d", got)
	}

	m = sendKey(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	if got := len(m.filteredTables()); got != 1 {
		t.Fatalf("expected 1 table after filter refinement, got %d", got)
	}

	selected, ok := m.selectedFilteredTable()
	if !ok {
		t.Fatal("expected a selected table after filtering")
	}
	if selected.Name != "orders" {
		t.Fatalf("expected selected table to be orders, got %q", selected.Name)
	}
}

func TestSelectingTableOpensQueryFlow(t *testing.T) {
	m := loadTablesForTest(t)

	m = sendKey(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	m = sendKey(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	m = sendKey(t, m, tea.KeyMsg{Type: tea.KeyEnter})

	if m.state != viewStateQuery {
		t.Fatalf("expected query view after opening selected table, got %q", m.state)
	}
	if m.activeTable != "users" {
		t.Fatalf("expected query flow for users, got %q", m.activeTable)
	}

	m = sendKey(t, m, tea.KeyMsg{Type: tea.KeyEsc})
	if m.state != viewStateTables {
		t.Fatalf("expected esc to return to table list, got %q", m.state)
	}
}

func TestTablesViewShowsColumnMetadata(t *testing.T) {
	m := loadTablesForTest(t)

	view := m.tablesView()
	required := []string{"Name", "Items", "Size", "Status", "orders", "ACTIVE"}
	for _, item := range required {
		if !strings.Contains(view, item) {
			t.Fatalf("expected tables view to include %q", item)
		}
	}
}

func loadTablesForTest(t *testing.T) Model {
	t.Helper()

	m := NewModel("dev", nil)
	tables := []aws.TableInfo{
		{Name: "orders", Status: "ACTIVE", ItemCount: 10, SizeBytes: 1_200, PartitionKey: "account_id", SortKey: "created_at"},
		{Name: "ops-audit", Status: "ACTIVE", ItemCount: 5, SizeBytes: 3_400, PartitionKey: "tenant_id", SortKey: "timestamp"},
		{Name: "users", Status: "ACTIVE", ItemCount: 7, SizeBytes: 9_100, PartitionKey: "org_id", SortKey: "user_id"},
	}

	next, _ := m.Update(tableLoadSuccessMsg{requestID: m.pendingTableLoadRequestID, tables: tables})
	loaded, ok := next.(Model)
	if !ok {
		t.Fatalf("expected Model, got %T", next)
	}

	return loaded
}

func sendKey(t *testing.T, m Model, msg tea.KeyMsg) Model {
	t.Helper()

	next, _ := m.Update(msg)
	updated, ok := next.(Model)
	if !ok {
		t.Fatalf("expected Model, got %T", next)
	}

	return updated
}
