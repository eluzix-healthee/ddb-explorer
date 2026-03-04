package tui

import (
	"errors"
	"strings"
	"testing"

	"ddb-explorer/aws"

	tea "github.com/charmbracelet/bubbletea"
)

func TestLoadTablesCmdReturnsTypedNilClientError(t *testing.T) {
	msg := loadTablesCmd(nil, 7)()
	errMsg, ok := msg.(tableLoadErrorMsg)
	if !ok {
		t.Fatalf("expected tableLoadErrorMsg, got %T", msg)
	}
	if errMsg.requestID != 7 {
		t.Fatalf("expected request id 7, got %d", errMsg.requestID)
	}
	if errMsg.err == nil || !strings.Contains(errMsg.err.Error(), "aws client is nil") {
		t.Fatalf("expected nil-client error, got %v", errMsg.err)
	}
}

func TestRunQueryCmdReturnsTypedNilClientError(t *testing.T) {
	request := queryRequest{
		tableName:      "orders",
		partitionKey:   "account_id",
		partitionValue: "acc-1",
	}

	msg := runQueryCmd(nil, request, 11)()
	errMsg, ok := msg.(queryErrorMsg)
	if !ok {
		t.Fatalf("expected queryErrorMsg, got %T", msg)
	}
	if errMsg.requestID != 11 {
		t.Fatalf("expected request id 11, got %d", errMsg.requestID)
	}
	if errMsg.tableName != "orders" {
		t.Fatalf("expected table name orders, got %q", errMsg.tableName)
	}
	if errMsg.err == nil || !strings.Contains(errMsg.err.Error(), "aws client is nil") {
		t.Fatalf("expected nil-client error, got %v", errMsg.err)
	}
}

func TestRunScanCmdReturnsTypedNilClientError(t *testing.T) {
	msg := runScanCmd(nil, "orders", 17)()
	errMsg, ok := msg.(scanErrorMsg)
	if !ok {
		t.Fatalf("expected scanErrorMsg, got %T", msg)
	}
	if errMsg.requestID != 17 {
		t.Fatalf("expected request id 17, got %d", errMsg.requestID)
	}
	if errMsg.tableName != "orders" {
		t.Fatalf("expected table name orders, got %q", errMsg.tableName)
	}
	if errMsg.err == nil || !strings.Contains(errMsg.err.Error(), "aws client is nil") {
		t.Fatalf("expected nil-client error, got %v", errMsg.err)
	}
}

func TestStaleQueryResponseIgnoredAfterSwitchingFlow(t *testing.T) {
	m := loadTablesForTest(t)
	m = sendKey(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	m.queryFields[queryFieldPartitionValue].value = "acc-1"
	m.queryFocus = m.queryInputCount()

	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected query command to be created")
	}
	running, ok := next.(Model)
	if !ok {
		t.Fatalf("expected Model, got %T", next)
	}
	if running.pendingQueryRequestID == 0 {
		t.Fatal("expected pending query request id")
	}
	requestID := running.pendingQueryRequestID

	running = sendKey(t, running, tea.KeyMsg{Type: tea.KeyCtrlS})
	if running.state != viewStateScan {
		t.Fatalf("expected scan state after switch, got %q", running.state)
	}

	next, _ = running.Update(querySuccessMsg{
		requestID: requestID,
		tableName: "orders",
		result:    aws.QueryResult{Items: []map[string]interface{}{{"account_id": "acc-1"}}},
	})
	updated, ok := next.(Model)
	if !ok {
		t.Fatalf("expected Model, got %T", next)
	}

	if updated.state != viewStateScan {
		t.Fatalf("expected stale query response to keep scan view, got %q", updated.state)
	}
	if !strings.Contains(updated.status, "ignored stale query response") {
		t.Fatalf("expected stale query status, got %q", updated.status)
	}
	if len(updated.resultItems) != 0 {
		t.Fatalf("expected stale query response to avoid result mutation, got %d items", len(updated.resultItems))
	}
}

func TestQueryErrorIsRecoverableInQueryView(t *testing.T) {
	m := loadTablesForTest(t)
	m = sendKey(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	m.queryFields[queryFieldPartitionValue].value = "acc-1"
	m.queryFocus = m.queryInputCount()

	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected query command to be created")
	}
	running, ok := next.(Model)
	if !ok {
		t.Fatalf("expected Model, got %T", next)
	}

	next, _ = running.Update(queryErrorMsg{
		requestID: running.pendingQueryRequestID,
		tableName: "orders",
		err:       errors.New("boom"),
	})
	updated, ok := next.(Model)
	if !ok {
		t.Fatalf("expected Model, got %T", next)
	}

	if updated.state != viewStateQuery {
		t.Fatalf("expected query view after recoverable error, got %q", updated.state)
	}
	if updated.err == nil || !strings.Contains(updated.err.Error(), "boom") {
		t.Fatalf("expected query error details, got %v", updated.err)
	}
	if !strings.Contains(updated.status, "query failed") {
		t.Fatalf("expected query failure status, got %q", updated.status)
	}
	if updated.pendingQueryRequestID != 0 {
		t.Fatalf("expected pending query request to clear, got %d", updated.pendingQueryRequestID)
	}
}
