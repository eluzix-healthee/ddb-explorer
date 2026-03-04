package tui

import (
	"fmt"
	"strings"
	"testing"

	"ddb-explorer/aws"

	tea "github.com/charmbracelet/bubbletea"
)

func TestCoreViewStateTransitionsAcrossPrimaryFlow(t *testing.T) {
	m := NewModel("dev", nil)
	tables := []aws.TableInfo{{
		Name:         "orders",
		Status:       "ACTIVE",
		PartitionKey: "account_id",
		SortKey:      "created_at",
	}}

	next, _ := m.Update(tableLoadSuccessMsg{requestID: m.pendingTableLoadRequestID, tables: tables})
	loaded, ok := next.(Model)
	if !ok {
		t.Fatalf("expected Model, got %T", next)
	}
	if loaded.state != viewStateTables {
		t.Fatalf("expected tables state after load success, got %q", loaded.state)
	}

	loaded = sendKey(t, loaded, tea.KeyMsg{Type: tea.KeyEnter})
	if loaded.state != viewStateQuery {
		t.Fatalf("expected query state after opening selected table, got %q", loaded.state)
	}

	loaded = sendKey(t, loaded, tea.KeyMsg{Type: tea.KeyCtrlS})
	if loaded.state != viewStateScan {
		t.Fatalf("expected scan state after switching from query, got %q", loaded.state)
	}

	loaded.pendingScanRequestID = 91
	next, _ = loaded.Update(scanSuccessMsg{
		requestID: 91,
		tableName: "orders",
		result: aws.QueryResult{Items: []map[string]interface{}{{
			"account_id": "acc-1",
			"status":     "OPEN",
		}}},
	})
	results, ok := next.(Model)
	if !ok {
		t.Fatalf("expected Model, got %T", next)
	}
	if results.state != viewStateResults {
		t.Fatalf("expected results state after scan success, got %q", results.state)
	}
	if results.resultOrigin != viewStateScan {
		t.Fatalf("expected results origin scan, got %q", results.resultOrigin)
	}

	results = sendKey(t, results, tea.KeyMsg{Type: tea.KeyEnter})
	if results.state != viewStateDetail {
		t.Fatalf("expected detail state after opening result row, got %q", results.state)
	}

	results = sendKey(t, results, tea.KeyMsg{Type: tea.KeyEsc})
	if results.state != viewStateResults {
		t.Fatalf("expected esc to return from detail to results, got %q", results.state)
	}

	results = sendKey(t, results, tea.KeyMsg{Type: tea.KeyEsc})
	if results.state != viewStateScan {
		t.Fatalf("expected esc to return from results to scan origin, got %q", results.state)
	}

	results = sendKey(t, results, tea.KeyMsg{Type: tea.KeyEsc})
	if results.state != viewStateTables {
		t.Fatalf("expected esc to return from scan to tables, got %q", results.state)
	}
}

func TestResizeRecalculatesResultsAndModalViewports(t *testing.T) {
	m := NewModel("dev", nil)
	items := make([]map[string]interface{}, 0, 20)
	for idx := 0; idx < 20; idx++ {
		items = append(items, map[string]interface{}{
			"id":    fmt.Sprintf("item-%02d", idx),
			"state": "ok",
		})
	}
	if err := m.enterResultsView("orders", aws.QueryResult{Items: items, RawItems: items}, viewStateQuery); err != nil {
		t.Fatalf("expected results view entry, got error: %v", err)
	}

	m.resultSelected = 15
	next, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 22})
	resized, ok := next.(Model)
	if !ok {
		t.Fatalf("expected Model, got %T", next)
	}
	if resized.resultPageSize() != 4 {
		t.Fatalf("expected result page size 4 at height 22, got %d", resized.resultPageSize())
	}
	if resized.resultPage != 3 {
		t.Fatalf("expected result page 3 for selected row 16, got %d", resized.resultPage)
	}

	next, _ = resized.Update(tea.WindowSizeMsg{Width: 100, Height: 20})
	resized, ok = next.(Model)
	if !ok {
		t.Fatalf("expected Model, got %T", next)
	}
	if resized.resultPageSize() != 2 {
		t.Fatalf("expected result page size 2 at height 20, got %d", resized.resultPageSize())
	}
	if resized.resultPage != 7 {
		t.Fatalf("expected result page 7 after second resize, got %d", resized.resultPage)
	}

	resized = sendKey(t, resized, tea.KeyMsg{Type: tea.KeyEnter})
	if resized.state != viewStateDetail {
		t.Fatalf("expected detail state, got %q", resized.state)
	}

	resized = sendKey(t, resized, tea.KeyMsg{Type: tea.KeyEnter})
	if !resized.jsonModalOpen {
		t.Fatal("expected JSON modal to open")
	}
	resized.jsonModalScroll = len(resized.jsonModalLines) + 20
	next, _ = resized.Update(tea.WindowSizeMsg{Width: 84, Height: 20})
	resized, ok = next.(Model)
	if !ok {
		t.Fatalf("expected Model, got %T", next)
	}
	maxScroll := len(resized.jsonModalLines) - resized.jsonModalPageSize()
	if maxScroll < 0 {
		maxScroll = 0
	}
	if resized.jsonModalScroll != maxScroll {
		t.Fatalf("expected modal scroll to clamp to %d after resize, got %d", maxScroll, resized.jsonModalScroll)
	}
}

func TestPrimaryViewsAndModalRemainCentered(t *testing.T) {
	assertCentered := func(t *testing.T, initial Model, marker string) {
		t.Helper()
		largeModel, _ := initial.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
		large, ok := largeModel.(Model)
		if !ok {
			t.Fatalf("expected Model, got %T", largeModel)
		}
		largeView := large.View()
		if got := len(strings.Split(largeView, "\n")); got != 40 {
			t.Fatalf("expected 40 lines after large resize, got %d", got)
		}
		largeIndex := lineIndexContaining(largeView, marker)
		if largeIndex == -1 {
			t.Fatalf("expected marker %q in large view", marker)
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
		smallIndex := lineIndexContaining(smallView, marker)
		if smallIndex == -1 {
			t.Fatalf("expected marker %q in small view", marker)
		}

		if largeIndex <= smallIndex {
			t.Fatalf("expected marker %q to move lower on taller viewport: large=%d small=%d", marker, largeIndex, smallIndex)
		}
	}

	tablesModel := loadTablesForTest(t)
	assertCentered(t, tablesModel, "Filter:")

	queryModel := sendKey(t, loadTablesForTest(t), tea.KeyMsg{Type: tea.KeyEnter})
	assertCentered(t, queryModel, "Partition Key Name")

	scanModel := sendKey(t, queryModel, tea.KeyMsg{Type: tea.KeyCtrlS})
	assertCentered(t, scanModel, "Scan uses the selected table")

	resultsModel := NewModel("dev", nil)
	if err := resultsModel.enterResultsView("orders", aws.QueryResult{Items: []map[string]interface{}{{
		"id":    "item-01",
		"state": "OPEN",
	}}}, viewStateQuery); err != nil {
		t.Fatalf("expected results view entry, got error: %v", err)
	}
	assertCentered(t, resultsModel, "Results")

	detailModel := sendKey(t, resultsModel, tea.KeyMsg{Type: tea.KeyEnter})
	assertCentered(t, detailModel, "Item Detail")

	modalModel := sendKey(t, detailModel, tea.KeyMsg{Type: tea.KeyEnter})
	assertCentered(t, modalModel, "Raw JSON Modal")
}
