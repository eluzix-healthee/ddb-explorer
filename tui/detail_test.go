package tui

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ddb-explorer/aws"

	tea "github.com/charmbracelet/bubbletea"
)

func TestResultsEnterOpensItemDetailView(t *testing.T) {
	m := NewModel("dev", nil)
	if err := m.enterResultsView("orders", aws.QueryResult{
		Items: []map[string]interface{}{
			{"account_id": "acc-1", "status": "OPEN", "notes": "short"},
		},
		RawItems: []map[string]interface{}{
			{"account_id": "acc-1", "status": "OPEN", "notes": "short"},
		},
	}, viewStateQuery); err != nil {
		t.Fatalf("expected results view entry, got error: %v", err)
	}

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
	if err := m.enterResultsView("orders", aws.QueryResult{
		Items:    []map[string]interface{}{{"account_id": "acc-1", "payload": "{...}"}},
		RawItems: []map[string]interface{}{raw},
	}, viewStateQuery); err != nil {
		t.Fatalf("expected results view entry, got error: %v", err)
	}

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

	if err := m.enterResultsView("orders", aws.QueryResult{
		Items:    []map[string]interface{}{{"account_id": "acc-1", "history": "[...]"}},
		RawItems: []map[string]interface{}{raw},
	}, viewStateQuery); err != nil {
		t.Fatalf("expected results view entry, got error: %v", err)
	}
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

func TestDetailViewClampsLargeValuesToViewport(t *testing.T) {
	m := NewModel("dev", nil)
	m.height = 20
	largeValue := strings.Repeat("x", 1200)

	if err := m.enterResultsView("orders", aws.QueryResult{
		Items: []map[string]interface{}{{
			"account_id": "acc-1",
			"payload":    largeValue,
			"status":     "OPEN",
		}},
		RawItems: []map[string]interface{}{{
			"account_id": "acc-1",
			"payload":    largeValue,
			"status":     "OPEN",
		}},
	}, viewStateQuery); err != nil {
		t.Fatalf("expected results view entry, got error: %v", err)
	}

	m = sendKey(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	view := m.View()
	if got := len(strings.Split(view, "\n")); got != m.height {
		t.Fatalf("expected detail view to stay within viewport height %d, got %d", m.height, got)
	}
	if !strings.Contains(view, "…") {
		t.Fatalf("expected truncated marker for oversized detail value, got %q", view)
	}
}

func TestSaveSelectedItemWritesJSONFile(t *testing.T) {
	m := NewModel("dev", nil)
	if err := m.enterResultsView("orders", aws.QueryResult{
		Items: []map[string]interface{}{{
			"account_id": "acc-1",
			"status":     "OPEN",
		}},
		RawItems: []map[string]interface{}{{
			"account_id": "acc-1",
			"status":     "OPEN",
		}},
	}, viewStateQuery); err != nil {
		t.Fatalf("expected results view entry, got error: %v", err)
	}

	m = sendKey(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	dir := t.TempDir()
	path, err := m.saveSelectedResultRawItemToDir(dir)
	if err != nil {
		t.Fatalf("expected save to succeed, got %v", err)
	}

	if filepath.Dir(path) != dir {
		t.Fatalf("expected file in temp dir %q, got %q", dir, path)
	}
	if !strings.HasSuffix(path, ".json") {
		t.Fatalf("expected JSON file suffix, got %q", path)
	}

	payload, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("expected to read saved file, got %v", err)
	}

	var decoded map[string]interface{}
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("expected valid JSON payload, got %v", err)
	}
	if decoded["account_id"] != "acc-1" {
		t.Fatalf("expected saved account_id acc-1, got %v", decoded["account_id"])
	}
}

func TestSaveKeyWorksInDetailAndJSONModal(t *testing.T) {
	m := NewModel("dev", nil)
	if err := m.enterResultsView("orders", aws.QueryResult{
		Items: []map[string]interface{}{{"account_id": "acc-1"}},
		RawItems: []map[string]interface{}{{
			"account_id": "acc-1",
			"payload":    map[string]interface{}{"status": "OPEN"},
		}},
	}, viewStateQuery); err != nil {
		t.Fatalf("expected results view entry, got error: %v", err)
	}

	m = sendKey(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	originalWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("resolve working directory: %v", err)
	}
	tempWD := t.TempDir()
	if err := os.Chdir(tempWD); err != nil {
		t.Fatalf("switch to temp dir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(originalWD)
	})

	m = sendKey(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	if !strings.Contains(m.status, "saved item JSON to") {
		t.Fatalf("expected save status in detail view, got %q", m.status)
	}

	m = sendKey(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	if !m.jsonModalOpen {
		t.Fatal("expected JSON modal to open")
	}
	m = sendKey(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	if !strings.Contains(m.status, "saved item JSON to") {
		t.Fatalf("expected save status in JSON modal, got %q", m.status)
	}
}

func TestJSONModalSearchFindsAndFocusesMatch(t *testing.T) {
	m := NewModel("dev", nil)
	m.height = 20

	lines := make([]interface{}, 0, 30)
	for idx := 0; idx < 30; idx++ {
		value := fmt.Sprintf("line-%02d", idx)
		if idx == 24 {
			value = "needle-target-value"
		}
		lines = append(lines, value)
	}

	if err := m.enterResultsView("orders", aws.QueryResult{
		Items:    []map[string]interface{}{{"payload": "[...]"}},
		RawItems: []map[string]interface{}{{"payload": lines}},
	}, viewStateQuery); err != nil {
		t.Fatalf("expected results view entry, got error: %v", err)
	}
	m = sendKey(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	m = sendKey(t, m, tea.KeyMsg{Type: tea.KeyEnter})

	m = sendKey(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	if !m.jsonSearchMode {
		t.Fatal("expected JSON search mode to open with '/'")
	}

	for _, r := range []rune("needle") {
		m = sendKey(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	m = sendKey(t, m, tea.KeyMsg{Type: tea.KeyEnter})

	if m.jsonSearchMode {
		t.Fatal("expected JSON search mode to close after Enter")
	}
	if len(m.jsonSearchMatches) == 0 {
		t.Fatal("expected at least one JSON search match")
	}
	if m.jsonModalScroll == 0 {
		t.Fatal("expected JSON modal scroll to move toward the match")
	}
	if !strings.Contains(m.status, "JSON match") {
		t.Fatalf("expected JSON match status, got %q", m.status)
	}
}

func TestJSONSearchMatchRangesFindsAllMatchesCaseInsensitive(t *testing.T) {
	line := "prefix Needle middle needle suffix"
	ranges := jsonSearchMatchRanges(line, "needle")

	if len(ranges) != 2 {
		t.Fatalf("expected two match ranges, got %d", len(ranges))
	}
	if got := line[ranges[0][0]:ranges[0][1]]; got != "Needle" {
		t.Fatalf("expected first match to preserve source casing, got %q", got)
	}
	if got := line[ranges[1][0]:ranges[1][1]]; got != "needle" {
		t.Fatalf("expected second match to preserve source casing, got %q", got)
	}
}
