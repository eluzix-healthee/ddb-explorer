package tui

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"ddb-explorer/aws"
	"ddb-explorer/tui/styles"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
)

type viewState string

const (
	viewStateLoading viewState = "loading"
	viewStateTables  viewState = "tables"
	viewStateQuery   viewState = "query"
	viewStateScan    viewState = "scan"
	viewStateResults viewState = "results"
	viewStateDetail  viewState = "detail"
	viewStateError   viewState = "error"
)

var allowedViewStateTransitions = map[viewState]map[viewState]struct{}{
	viewStateLoading: {
		viewStateTables:  {},
		viewStateResults: {},
		viewStateError:   {},
	},
	viewStateTables: {
		viewStateLoading: {},
		viewStateQuery:   {},
		viewStateError:   {},
	},
	viewStateQuery: {
		viewStateTables:  {},
		viewStateScan:    {},
		viewStateResults: {},
		viewStateError:   {},
	},
	viewStateScan: {
		viewStateTables:  {},
		viewStateQuery:   {},
		viewStateResults: {},
		viewStateError:   {},
	},
	viewStateResults: {
		viewStateQuery:  {},
		viewStateScan:   {},
		viewStateDetail: {},
		viewStateError:  {},
	},
	viewStateDetail: {
		viewStateResults: {},
		viewStateError:   {},
	},
	viewStateError: {
		viewStateLoading: {},
		viewStateTables:  {},
		viewStateError:   {},
	},
}

const (
	queryFieldPartitionValue = iota
	queryFieldSortCondition
	queryFieldSortValue
	queryFieldSortValueEnd
)

var querySortKeyConditions = []string{"=", "begins_with", "<", "<=", ">", ">=", "between"}

type queryField struct {
	label       string
	value       string
	placeholder string
	required    bool
}

type queryRequest struct {
	tableName      string
	partitionKey   string
	partitionValue string
	sortKey        string
	condition      string
	sortValue      string
	sortValueEnd   string
	indexName      string
}

type queryTarget struct {
	label        string
	partitionKey string
	sortKey      string
	indexName    string
}

type Model struct {
	profile string
	client  *aws.Client
	width   int
	height  int
	state   viewState
	tables  []aws.TableInfo
	status  string
	err     error

	nextRequestID             uint64
	pendingTableLoadRequestID uint64
	pendingQueryRequestID     uint64
	pendingScanRequestID      uint64

	tableFilter         string
	filterInputActive   bool
	selectedTable       int
	activeTable         string
	queryFields         []queryField
	queryFocus          int
	queryTargets        []queryTarget
	selectedQueryTarget int
	resultItems         []map[string]interface{}
	resultRawItems      []map[string]interface{}
	resultColumns       []string
	resultSelected      int
	resultPage          int
	resultOrigin        viewState
	resultHasMore       bool
	detailSelected      int
	detailScroll        int
	jsonModalOpen       bool
	jsonModalLines      []string
	jsonModalScroll     int
	jsonSearchMode      bool
	jsonSearchInput     string
	jsonSearchQuery     string
	jsonSearchMatches   []int
	jsonSearchCurrent   int

	showHelp bool
	spinner  spinner.Model
	keys     KeyMap
	theme    styles.Theme
}

func NewModel(profile string, client *aws.Client) Model {
	theme := styles.Default()
	spin := spinner.New()
	spin.Spinner = spinner.Dot
	spin.Style = theme.Highlight

	return Model{
		profile:                   profile,
		client:                    client,
		width:                     80,
		height:                    24,
		state:                     viewStateLoading,
		status:                    "loading DynamoDB tables",
		nextRequestID:             1,
		pendingTableLoadRequestID: 1,
		queryFields:               defaultQueryFields(),
		spinner:                   spin,
		keys:                      defaultKeyMap(),
		theme:                     theme,
	}
}

func NewProgram(model Model) *tea.Program {
	return tea.NewProgram(model, tea.WithAltScreen())
}

func (m Model) Init() tea.Cmd {
	return m.loadTablesWithSpinnerCmd(m.pendingTableLoadRequestID)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.clampResultsViewport()
		m.clampDetailViewport()
		m.clampJSONModalViewport()
		return m, nil
	case tea.KeyMsg:
		if key.Matches(msg, m.keys.Global.Quit) {
			return m, tea.Quit
		}
		if m.state == viewStateDetail && m.jsonModalOpen && m.jsonSearchMode && key.Matches(msg, m.keys.Global.Back) {
			m.jsonSearchMode = false
			m.status = "closed JSON search"
			return m, nil
		}
		if m.showHelp {
			if key.Matches(msg, m.keys.Global.Help) || key.Matches(msg, m.keys.Global.Back) {
				m.showHelp = false
				m.status = "closed keyboard shortcuts"
			}
			return m, nil
		}
		if key.Matches(msg, m.keys.Global.Help) {
			m.showHelp = true
			m.status = "opened keyboard shortcuts"
			return m, nil
		}
		if key.Matches(msg, m.keys.Global.Back) {
			return m.handleBackKey()
		}

		switch m.state {
		case viewStateTables:
			return m.updateTablesKey(msg)
		case viewStateQuery:
			return m.updateQueryKey(msg)
		case viewStateScan:
			return m.updateScanKey(msg)
		case viewStateResults:
			return m.updateResultsKey(msg)
		case viewStateDetail:
			return m.updateDetailKey(msg)
		}
	case tableLoadSuccessMsg:
		if !m.isExpectedTableLoadResponse(msg.requestID) {
			m.status = staleResponseStatus("table load", msg.requestID)
			return m, nil
		}
		m.pendingTableLoadRequestID = 0
		m.tables = msg.tables
		if err := m.transitionTo(viewStateTables); err != nil {
			m.transitionFailure(err)
			return m, nil
		}
		m.activeTable = ""
		m.filterInputActive = false
		m.selectedTable = 0
		visible := len(m.filteredTables())
		m.status = fmt.Sprintf("loaded %d tables (%d visible) with profile %s", len(msg.tables), visible, m.profile)
		m.err = nil
		return m, nil
	case tableLoadErrorMsg:
		if !m.isExpectedTableLoadResponse(msg.requestID) {
			m.status = staleResponseStatus("table load", msg.requestID)
			return m, nil
		}
		m.pendingTableLoadRequestID = 0
		if err := m.transitionTo(viewStateError); err != nil {
			m.transitionFailure(err)
			return m, nil
		}
		m.status = "failed to load tables"
		m.err = normalizeError(msg.err, "table load failed")
		return m, nil
	case querySuccessMsg:
		if !m.isExpectedQueryResponse(msg.requestID) {
			m.status = staleResponseStatus("query", msg.requestID)
			return m, nil
		}
		m.pendingQueryRequestID = 0
		if err := m.enterResultsView(msg.tableName, msg.result, viewStateQuery); err != nil {
			m.transitionFailure(err)
			return m, nil
		}
		m.status = fmt.Sprintf("query returned %d items from %s", len(msg.result.Items), msg.tableName)
		m.err = nil
		return m, nil
	case queryErrorMsg:
		if !m.isExpectedQueryResponse(msg.requestID) {
			m.status = staleResponseStatus("query", msg.requestID)
			return m, nil
		}
		m.pendingQueryRequestID = 0
		m.status = fmt.Sprintf("query failed for %s", msg.tableName)
		m.err = normalizeError(msg.err, "query failed")
		return m, nil
	case scanSuccessMsg:
		if !m.isExpectedScanResponse(msg.requestID) {
			m.status = staleResponseStatus("scan", msg.requestID)
			return m, nil
		}
		m.pendingScanRequestID = 0
		if err := m.enterResultsView(msg.tableName, msg.result, viewStateScan); err != nil {
			m.transitionFailure(err)
			return m, nil
		}
		m.status = fmt.Sprintf("scan returned %d items from %s", len(msg.result.Items), msg.tableName)
		m.err = nil
		return m, nil
	case scanErrorMsg:
		if !m.isExpectedScanResponse(msg.requestID) {
			m.status = staleResponseStatus("scan", msg.requestID)
			return m, nil
		}
		m.pendingScanRequestID = 0
		m.status = fmt.Sprintf("scan failed for %s", msg.tableName)
		m.err = normalizeError(msg.err, "scan failed")
		return m, nil
	case spinner.TickMsg:
		if m.state != viewStateLoading {
			return m, nil
		}

		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m Model) View() string {
	var content string
	switch m.state {
	case viewStateLoading:
		content = m.loadingView()
	case viewStateTables:
		content = m.tablesView()
	case viewStateQuery:
		content = m.queryView()
	case viewStateScan:
		content = m.scanView()
	case viewStateResults:
		content = m.resultsView()
	case viewStateDetail:
		content = m.detailView()
	case viewStateError:
		content = m.errorView()
	default:
		content = m.theme.Error.Render("invalid app state")
	}
	if m.showHelp {
		content = m.helpOverlayView()
	}

	return m.renderChrome(content)
}

func (m Model) loadTablesWithSpinnerCmd(requestID uint64) tea.Cmd {
	return tea.Batch(m.spinner.Tick, loadTablesCmd(m.client, requestID))
}

func loadTablesCmd(client *aws.Client, requestID uint64) tea.Cmd {
	return func() tea.Msg {
		if client == nil {
			return tableLoadErrorMsg{requestID: requestID, err: errors.New("aws client is nil")}
		}

		tables, err := client.ListTables()
		if err != nil {
			return tableLoadErrorMsg{requestID: requestID, err: err}
		}

		return tableLoadSuccessMsg{requestID: requestID, tables: tables}
	}
}

func normalizeError(err error, fallback string) error {
	if err != nil {
		return err
	}

	return errors.New(fallback)
}

func staleResponseStatus(operation string, requestID uint64) string {
	if requestID == 0 {
		return fmt.Sprintf("ignored stale %s response", operation)
	}

	return fmt.Sprintf("ignored stale %s response #%d", operation, requestID)
}

func (m *Model) reserveRequestID() uint64 {
	m.nextRequestID++
	return m.nextRequestID
}

func (m Model) isExpectedTableLoadResponse(requestID uint64) bool {
	return m.state == viewStateLoading && requestID != 0 && requestID == m.pendingTableLoadRequestID
}

func (m Model) isExpectedQueryResponse(requestID uint64) bool {
	return m.state == viewStateQuery && requestID != 0 && requestID == m.pendingQueryRequestID
}

func (m Model) isExpectedScanResponse(requestID uint64) bool {
	return m.state == viewStateScan && requestID != 0 && requestID == m.pendingScanRequestID
}

func validateStateTransition(currentState viewState, nextState viewState) error {
	if currentState == nextState {
		if _, ok := allowedViewStateTransitions[currentState]; ok {
			return nil
		}
		return fmt.Errorf("unknown current state %q", currentState)
	}

	allowedNextStates, ok := allowedViewStateTransitions[currentState]
	if !ok {
		return fmt.Errorf("unknown current state %q", currentState)
	}
	if _, ok := allowedViewStateTransitions[nextState]; !ok {
		return fmt.Errorf("unknown next state %q", nextState)
	}

	if _, ok := allowedNextStates[nextState]; !ok {
		return fmt.Errorf("invalid view transition %q -> %q", currentState, nextState)
	}

	return nil
}

func (m *Model) transitionTo(nextState viewState) error {
	if err := validateStateTransition(m.state, nextState); err != nil {
		return err
	}

	m.state = nextState
	return nil
}

func (m *Model) transitionFailure(err error) {
	m.state = viewStateError
	m.status = "internal state transition error"
	m.err = err
	m.pendingTableLoadRequestID = 0
	m.pendingQueryRequestID = 0
	m.pendingScanRequestID = 0
}

func (m Model) handleBackKey() (tea.Model, tea.Cmd) {
	switch m.state {
	case viewStateTables:
		if m.filterInputActive {
			m.filterInputActive = false
			m.status = "stopped editing table filter"
			return m, nil
		}
		m.status = "already at root view"
		return m, nil
	case viewStateQuery, viewStateScan:
		if m.state == viewStateQuery {
			m.pendingQueryRequestID = 0
		}
		if m.state == viewStateScan {
			m.pendingScanRequestID = 0
		}
		if err := m.transitionTo(viewStateTables); err != nil {
			m.transitionFailure(err)
			return m, nil
		}
		m.status = "returned to table list"
		return m, nil
	case viewStateResults:
		if m.jsonModalOpen {
			m.jsonModalOpen = false
			m.jsonSearchMode = false
			m.jsonSearchInput = ""
			m.status = "closed JSON modal"
			return m, nil
		}
		if m.resultOrigin == viewStateScan {
			if err := m.transitionTo(viewStateScan); err != nil {
				m.transitionFailure(err)
				return m, nil
			}
			m.status = "returned to scan form"
			return m, nil
		}

		if err := m.transitionTo(viewStateQuery); err != nil {
			m.transitionFailure(err)
			return m, nil
		}
		m.status = "returned to query form"
		return m, nil
	case viewStateDetail:
		if m.jsonModalOpen {
			m.jsonModalOpen = false
			m.jsonSearchMode = false
			m.jsonSearchInput = ""
			m.status = "closed JSON modal"
			return m, nil
		}
		if err := m.transitionTo(viewStateResults); err != nil {
			m.transitionFailure(err)
			return m, nil
		}
		m.status = "returned to results table"
		return m, nil
	case viewStateError:
		if len(m.tables) > 0 {
			if err := m.transitionTo(viewStateTables); err != nil {
				m.transitionFailure(err)
				return m, nil
			}
			m.status = "returned to table list"
			m.err = nil
			return m, nil
		}
		m.status = "already at root view"
		return m, nil
	default:
		m.status = "already at root view"
		return m, nil
	}
}

func (m Model) updateTablesKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.filterInputActive {
		next, handled := m.updateFilterInput(msg)
		if handled {
			return next, nil
		}
	}

	if key.Matches(msg, m.keys.TableList.Refresh) {
		if err := m.transitionTo(viewStateLoading); err != nil {
			m.transitionFailure(err)
			return m, nil
		}
		requestID := m.reserveRequestID()
		m.pendingTableLoadRequestID = requestID
		m.pendingQueryRequestID = 0
		m.pendingScanRequestID = 0
		m.status = "refreshing table list"
		m.err = nil
		m.filterInputActive = false
		return m, m.loadTablesWithSpinnerCmd(requestID)
	}
	if key.Matches(msg, m.keys.TableList.Filter) {
		m.filterInputActive = true
		m.status = fmt.Sprintf("editing filter (%d visible)", len(m.filteredTables()))
		return m, nil
	}
	if key.Matches(msg, m.keys.TableList.MoveUp) {
		m.moveSelection(-1)
		return m, nil
	}
	if key.Matches(msg, m.keys.TableList.MoveDown) {
		m.moveSelection(1)
		return m, nil
	}
	if key.Matches(msg, m.keys.TableList.Open) {
		table, ok := m.selectedFilteredTable()
		if !ok {
			m.status = "no table selected"
			return m, nil
		}

		if err := m.enterQueryFlow(table); err != nil {
			m.transitionFailure(err)
			return m, nil
		}
		m.filterInputActive = false
		m.status = fmt.Sprintf("opened query flow for %s", table.Name)
		return m, nil
	}

	return m, nil
}

func (m Model) updateQueryKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if key.Matches(msg, m.keys.Query.CycleTarget) {
		if m.cycleQueryTarget(1) {
			m.status = fmt.Sprintf("query source set to %s", m.currentQueryTargetLabel())
		}
		return m, nil
	}

	if m.queryFocus == queryFieldSortCondition {
		if key.Matches(msg, m.keys.Query.NextOption) {
			m.cycleSortCondition(1)
			m.status = fmt.Sprintf("sort key condition set to %s", m.queryFields[queryFieldSortCondition].value)
			return m, nil
		}
		if key.Matches(msg, m.keys.Query.PrevOption) {
			m.cycleSortCondition(-1)
			m.status = fmt.Sprintf("sort key condition set to %s", m.queryFields[queryFieldSortCondition].value)
			return m, nil
		}
	}

	if key.Matches(msg, m.keys.Query.SwitchScan) {
		m.pendingQueryRequestID = 0
		if err := m.transitionTo(viewStateScan); err != nil {
			m.transitionFailure(err)
			return m, nil
		}
		m.status = fmt.Sprintf("switched to scan flow for %s", m.activeTable)
		return m, nil
	}
	if key.Matches(msg, m.keys.Query.NextField) {
		m.shiftQueryFocus(1)
		m.status = fmt.Sprintf("focused %s", m.queryFocusLabel())
		return m, nil
	}
	if key.Matches(msg, m.keys.Query.PrevField) {
		m.shiftQueryFocus(-1)
		m.status = fmt.Sprintf("focused %s", m.queryFocusLabel())
		return m, nil
	}
	if key.Matches(msg, m.keys.Query.Submit) {
		if m.queryFocus < m.queryInputCount() {
			m.shiftQueryFocus(1)
			m.status = fmt.Sprintf("focused %s", m.queryFocusLabel())
			return m, nil
		}

		request, err := m.buildQueryRequest()
		if err != nil {
			m.status = err.Error()
			m.err = err
			return m, nil
		}

		m.status = fmt.Sprintf("running query for %s", request.tableName)
		m.err = nil
		requestID := m.reserveRequestID()
		m.pendingQueryRequestID = requestID
		m.pendingScanRequestID = 0
		return m, runQueryCmd(m.client, request, requestID)
	}

	if m.queryFocus >= m.queryInputCount() {
		return m, nil
	}

	if m.updateQueryInput(msg) {
		m.status = fmt.Sprintf("editing %s", m.queryFields[m.queryFocus].label)
		return m, nil
	}

	return m, nil
}

func (m Model) updateScanKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if key.Matches(msg, m.keys.Scan.Run) {
		if m.activeTable == "" {
			m.status = "cannot run scan without selected table"
			return m, nil
		}

		m.status = fmt.Sprintf("running scan for %s", m.activeTable)
		m.err = nil
		requestID := m.reserveRequestID()
		m.pendingScanRequestID = requestID
		m.pendingQueryRequestID = 0
		return m, runScanCmd(m.client, m.activeTable, requestID)
	}
	return m, nil
}

func (m Model) updateResultsKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if key.Matches(msg, m.keys.Results.MoveUp) {
		m.moveResultSelection(-1)
		return m, nil
	}
	if key.Matches(msg, m.keys.Results.MoveDown) {
		m.moveResultSelection(1)
		return m, nil
	}
	if key.Matches(msg, m.keys.Results.NextPage) {
		m.moveResultPage(1)
		return m, nil
	}
	if key.Matches(msg, m.keys.Results.PrevPage) {
		m.moveResultPage(-1)
		return m, nil
	}
	if key.Matches(msg, m.keys.Results.Open) {
		if err := m.openSelectedResultDetail(); err != nil {
			m.status = err.Error()
			m.err = err
			return m, nil
		}

		return m, nil
	}

	return m, nil
}

func (m Model) updateDetailKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.jsonModalOpen {
		if m.jsonSearchMode {
			return m.updateJSONSearchInput(msg), nil
		}
		if key.Matches(msg, m.keys.Detail.SaveItem) {
			path, err := m.saveSelectedResultRawItemToCurrentDir()
			if err != nil {
				m.status = err.Error()
				m.err = err
				return m, nil
			}

			m.status = fmt.Sprintf("saved item JSON to %s", path)
			m.err = nil
			return m, nil
		}
		if key.Matches(msg, m.keys.Detail.OpenJSON) {
			m.jsonModalOpen = false
			m.jsonSearchMode = false
			m.jsonSearchInput = ""
			m.status = "closed JSON modal"
			return m, nil
		}
		if key.Matches(msg, m.keys.Detail.Search) {
			m.jsonSearchMode = true
			m.jsonSearchInput = m.jsonSearchQuery
			m.status = "search JSON"
			return m, nil
		}
		if key.Matches(msg, m.keys.Detail.MoveUp) {
			m.moveJSONModalScroll(-1)
			return m, nil
		}
		if key.Matches(msg, m.keys.Detail.MoveDown) {
			m.moveJSONModalScroll(1)
			return m, nil
		}
		if key.Matches(msg, m.keys.Detail.NextPage) {
			m.moveJSONModalScroll(m.jsonModalPageSize())
			return m, nil
		}
		if key.Matches(msg, m.keys.Detail.PrevPage) {
			m.moveJSONModalScroll(-m.jsonModalPageSize())
			return m, nil
		}

		return m, nil
	}

	if key.Matches(msg, m.keys.Detail.MoveUp) {
		m.moveDetailSelection(-1)
		return m, nil
	}
	if key.Matches(msg, m.keys.Detail.MoveDown) {
		m.moveDetailSelection(1)
		return m, nil
	}
	if key.Matches(msg, m.keys.Detail.OpenJSON) {
		if err := m.openJSONModal(); err != nil {
			m.status = err.Error()
			m.err = err
			return m, nil
		}

		m.err = nil
		return m, nil
	}
	if key.Matches(msg, m.keys.Detail.SaveItem) {
		path, err := m.saveSelectedResultRawItemToCurrentDir()
		if err != nil {
			m.status = err.Error()
			m.err = err
			return m, nil
		}

		m.status = fmt.Sprintf("saved item JSON to %s", path)
		m.err = nil
		return m, nil
	}

	return m, nil
}

func (m Model) updateFilterInput(msg tea.KeyMsg) (Model, bool) {
	switch msg.Type {
	case tea.KeyEnter:
		m.filterInputActive = false
		m.status = fmt.Sprintf("applied filter (%d visible)", len(m.filteredTables()))
		return m, true
	case tea.KeyBackspace, tea.KeyDelete:
		if m.tableFilter == "" {
			m.status = fmt.Sprintf("editing filter (%d visible)", len(m.filteredTables()))
			return m, true
		}
		runes := []rune(m.tableFilter)
		m.tableFilter = string(runes[:len(runes)-1])
	case tea.KeyRunes:
		m.tableFilter += string(msg.Runes)
	default:
		return m, false
	}

	m.selectedTable = 0
	m.status = fmt.Sprintf("filter %q (%d visible)", m.tableFilter, len(m.filteredTables()))
	return m, true
}

func (m *Model) moveSelection(delta int) {
	visible := len(m.filteredTables())
	if visible == 0 {
		m.selectedTable = 0
		m.status = "no tables match current filter"
		return
	}

	m.selectedTable += delta
	if m.selectedTable < 0 {
		m.selectedTable = 0
	}
	if m.selectedTable >= visible {
		m.selectedTable = visible - 1
	}

	table := m.filteredTables()[m.selectedTable]
	m.status = fmt.Sprintf("selected table %s (%d/%d)", table.Name, m.selectedTable+1, visible)
}

func (m Model) selectedFilteredTable() (aws.TableInfo, bool) {
	visible := m.filteredTables()
	if len(visible) == 0 {
		return aws.TableInfo{}, false
	}

	index := m.selectedTable
	if index < 0 {
		index = 0
	}
	if index >= len(visible) {
		index = len(visible) - 1
	}

	return visible[index], true
}

func (m Model) filteredTables() []aws.TableInfo {
	filter := strings.TrimSpace(strings.ToLower(m.tableFilter))
	if filter == "" {
		return m.tables
	}

	filtered := make([]aws.TableInfo, 0, len(m.tables))
	for _, table := range m.tables {
		if strings.Contains(strings.ToLower(table.Name), filter) || strings.Contains(strings.ToLower(table.Status), filter) {
			filtered = append(filtered, table)
		}
	}

	return filtered
}

func (m *Model) enterQueryFlow(table aws.TableInfo) error {
	if err := m.transitionTo(viewStateQuery); err != nil {
		return err
	}
	m.activeTable = table.Name
	m.resultItems = nil
	m.resultRawItems = nil
	m.resultColumns = nil
	m.resultSelected = 0
	m.resultPage = 0
	m.resultOrigin = ""
	m.resultHasMore = false
	m.pendingQueryRequestID = 0
	m.pendingScanRequestID = 0
	m.detailSelected = 0
	m.detailScroll = 0
	m.jsonModalOpen = false
	m.jsonModalLines = nil
	m.jsonModalScroll = 0
	m.queryTargets = buildQueryTargets(table)
	m.selectedQueryTarget = 0
	m.queryFields = defaultQueryFields()
	m.refreshQueryFieldLabels()
	m.queryFocus = 0

	return nil
}

func (m *Model) enterResultsView(tableName string, result aws.QueryResult, origin viewState) error {
	if err := m.transitionTo(viewStateResults); err != nil {
		return err
	}
	m.activeTable = tableName
	m.resultItems = result.Items
	if len(result.RawItems) == len(result.Items) {
		m.resultRawItems = result.RawItems
	} else {
		m.resultRawItems = make([]map[string]interface{}, len(result.Items))
		copy(m.resultRawItems, result.Items)
	}
	m.resultColumns = m.discoverResultColumnsForOrigin(result.Items, origin)
	m.resultSelected = 0
	m.resultPage = 0
	m.resultOrigin = origin
	m.resultHasMore = len(result.LastEvaluatedKey) > 0
	m.detailSelected = 0
	m.detailScroll = 0
	m.jsonModalOpen = false
	m.jsonModalLines = nil
	m.jsonModalScroll = 0
	m.clampResultsViewport()

	return nil
}

func discoverResultColumns(items []map[string]interface{}) []string {
	if len(items) == 0 {
		return nil
	}

	unique := make(map[string]struct{})
	for _, item := range items {
		for key := range item {
			unique[key] = struct{}{}
		}
	}

	columns := make([]string, 0, len(unique))
	for key := range unique {
		columns = append(columns, key)
	}
	sort.Strings(columns)

	return columns
}

func (m Model) discoverResultColumnsForOrigin(items []map[string]interface{}, _ viewState) []string {
	columns := discoverResultColumns(items)
	filtered := make([]string, 0, len(columns))
	for _, column := range columns {
		if strings.HasPrefix(strings.ToUpper(column), "GSI") {
			continue
		}
		filtered = append(filtered, column)
	}

	if len(filtered) == 0 {
		return columns
	}

	return filtered
}

func (m Model) chromeContentHeight() int {
	renderHeight := m.height
	if renderHeight < 4 {
		renderHeight = 4
	}

	contentHeight := renderHeight - 3
	if contentHeight < 1 {
		return 1
	}

	return contentHeight
}

func (m Model) stretchedPanelSize() (int, int) {
	panelWidth := m.width - 4
	if panelWidth < 32 {
		panelWidth = 32
	}

	panelHeight := m.chromeContentHeight()
	if panelHeight < 10 {
		panelHeight = 10
	}

	return panelWidth, panelHeight
}

func (m Model) stretchedPanelContentSize() (int, int) {
	panelWidth, panelHeight := m.stretchedPanelSize()
	contentWidth := panelWidth - m.theme.Panel.GetHorizontalFrameSize()
	contentHeight := panelHeight - m.theme.Panel.GetVerticalFrameSize()
	if contentWidth < 1 {
		contentWidth = 1
	}
	if contentHeight < 1 {
		contentHeight = 1
	}

	return contentWidth, contentHeight
}

func (m Model) resultPageSize() int {
	_, contentHeight := m.stretchedPanelContentSize()
	pageSize := contentHeight - 7
	if pageSize < 1 {
		return 1
	}

	return pageSize
}

func (m Model) resultPageCount() int {
	if len(m.resultItems) == 0 {
		return 1
	}

	pageSize := m.resultPageSize()
	return (len(m.resultItems)-1)/pageSize + 1
}

func (m Model) currentResultPageRange() (int, int) {
	if len(m.resultItems) == 0 {
		return 0, 0
	}

	pageSize := m.resultPageSize()
	start := m.resultPage * pageSize
	if start < 0 {
		start = 0
	}
	if start >= len(m.resultItems) {
		start = (len(m.resultItems) - 1) / pageSize * pageSize
	}

	end := start + pageSize
	if end > len(m.resultItems) {
		end = len(m.resultItems)
	}

	return start, end
}

func (m *Model) clampResultsViewport() {
	if len(m.resultItems) == 0 {
		m.resultSelected = 0
		m.resultPage = 0
		return
	}

	if m.resultSelected < 0 {
		m.resultSelected = 0
	}
	if m.resultSelected >= len(m.resultItems) {
		m.resultSelected = len(m.resultItems) - 1
	}

	pageSize := m.resultPageSize()
	if pageSize <= 0 {
		m.resultPage = 0
		return
	}

	m.resultPage = m.resultSelected / pageSize
}

func (m *Model) moveResultSelection(delta int) {
	if len(m.resultItems) == 0 {
		m.resultSelected = 0
		m.resultPage = 0
		m.status = "no rows available"
		return
	}

	m.resultSelected += delta
	if m.resultSelected < 0 {
		m.resultSelected = 0
	}
	if m.resultSelected >= len(m.resultItems) {
		m.resultSelected = len(m.resultItems) - 1
	}

	m.clampResultsViewport()
	m.status = fmt.Sprintf("row %d of %d", m.resultSelected+1, len(m.resultItems))
}

func (m *Model) moveResultPage(delta int) {
	if len(m.resultItems) == 0 {
		m.status = "no rows available"
		m.resultPage = 0
		m.resultSelected = 0
		return
	}

	pageCount := m.resultPageCount()
	if pageCount <= 1 {
		m.status = "already on the only page"
		return
	}

	currentStart, _ := m.currentResultPageRange()
	offset := m.resultSelected - currentStart
	if offset < 0 {
		offset = 0
	}

	nextPage := m.resultPage + delta
	if nextPage < 0 {
		nextPage = 0
	}
	if nextPage >= pageCount {
		nextPage = pageCount - 1
	}
	m.resultPage = nextPage

	start, end := m.currentResultPageRange()
	target := start + offset
	if target >= end {
		target = end - 1
	}
	if target < start {
		target = start
	}
	m.resultSelected = target

	m.status = m.resultsPaginationStatus()
}

func (m Model) resultsPaginationStatus() string {
	if len(m.resultItems) == 0 {
		return "Page 1/1 | rows 0-0 of 0"
	}

	start, end := m.currentResultPageRange()
	return fmt.Sprintf(
		"Page %d/%d | rows %d-%d of %d",
		m.resultPage+1,
		m.resultPageCount(),
		start+1,
		end,
		len(m.resultItems),
	)
}

func (m *Model) shiftQueryFocus(delta int) {
	max := m.queryInputCount()
	if max < 0 {
		max = 0
	}

	next := m.queryFocus + delta
	if next < 0 {
		next = max
	}
	if next > max {
		next = 0
	}

	m.queryFocus = next
}

func buildQueryTargets(table aws.TableInfo) []queryTarget {
	targets := []queryTarget{{
		label:        "Table",
		partitionKey: table.PartitionKey,
		sortKey:      table.SortKey,
	}}

	for _, gsi := range table.GSIKeys {
		targets = append(targets, queryTarget{
			label:        "GSI: " + gsi.Name,
			partitionKey: gsi.PartitionKey,
			sortKey:      gsi.SortKey,
			indexName:    gsi.Name,
		})
	}

	return targets
}

func (m Model) currentQueryTarget() (queryTarget, bool) {
	if len(m.queryTargets) == 0 {
		return queryTarget{}, false
	}

	index := m.selectedQueryTarget
	if index < 0 {
		index = 0
	}
	if index >= len(m.queryTargets) {
		index = len(m.queryTargets) - 1
	}

	return m.queryTargets[index], true
}

func (m Model) currentQueryTargetLabel() string {
	target, ok := m.currentQueryTarget()
	if !ok {
		return "(none)"
	}

	if target.sortKey == "" {
		return fmt.Sprintf("%s [%s]", target.label, target.partitionKey)
	}

	return fmt.Sprintf("%s [%s, %s]", target.label, target.partitionKey, target.sortKey)
}

func (m *Model) cycleQueryTarget(delta int) bool {
	if len(m.queryTargets) <= 1 {
		return false
	}

	next := m.selectedQueryTarget + delta
	if next < 0 {
		next = len(m.queryTargets) - 1
	}
	if next >= len(m.queryTargets) {
		next = 0
	}

	m.selectedQueryTarget = next
	m.refreshQueryFieldLabels()
	return true
}

func (m *Model) refreshQueryFieldLabels() {
	target, ok := m.currentQueryTarget()
	if !ok || len(m.queryFields) <= queryFieldSortValueEnd {
		return
	}

	m.queryFields[queryFieldPartitionValue].label = fmt.Sprintf("%s Value", target.partitionKey)
	m.queryFields[queryFieldPartitionValue].placeholder = fmt.Sprintf("value for %s", target.partitionKey)

	if target.sortKey == "" {
		m.queryFields[queryFieldSortCondition].label = "Sort Key Condition"
		m.queryFields[queryFieldSortValue].label = "Sort Key Value"
		m.queryFields[queryFieldSortValue].placeholder = "selected source has no sort key"
		m.queryFields[queryFieldSortValue].value = ""
		m.queryFields[queryFieldSortValueEnd].label = "Sort Key Value (End)"
		m.queryFields[queryFieldSortValueEnd].placeholder = "selected source has no sort key"
		m.queryFields[queryFieldSortValueEnd].value = ""
		return
	}

	m.queryFields[queryFieldSortCondition].label = fmt.Sprintf("%s Condition", target.sortKey)
	m.queryFields[queryFieldSortValue].label = fmt.Sprintf("%s Value", target.sortKey)
	m.queryFields[queryFieldSortValue].placeholder = fmt.Sprintf("optional value for %s", target.sortKey)
	m.queryFields[queryFieldSortValueEnd].label = fmt.Sprintf("%s Value (End)", target.sortKey)
	m.queryFields[queryFieldSortValueEnd].placeholder = fmt.Sprintf("required end value for %s when condition is between", target.sortKey)

	if m.queryFields[queryFieldSortCondition].value != "between" {
		m.queryFields[queryFieldSortValueEnd].value = ""
	}
}

func (m Model) queryInputCount() int {
	target, ok := m.currentQueryTarget()
	if !ok {
		return 1
	}
	if target.sortKey == "" {
		return 1
	}
	if len(m.queryFields) <= queryFieldSortValueEnd {
		return len(m.queryFields)
	}
	if strings.TrimSpace(m.queryFields[queryFieldSortCondition].value) == "between" {
		return queryFieldSortValueEnd + 1
	}

	return queryFieldSortValue + 1
}

func sortConditionIndex(condition string) int {
	for idx, value := range querySortKeyConditions {
		if value == condition {
			return idx
		}
	}

	return 0
}

func isValidSortCondition(condition string) bool {
	for _, value := range querySortKeyConditions {
		if value == condition {
			return true
		}
	}

	return false
}

func (m *Model) cycleSortCondition(delta int) {
	target, ok := m.currentQueryTarget()
	if !ok || target.sortKey == "" {
		return
	}
	if len(m.queryFields) <= queryFieldSortCondition {
		return
	}

	current := sortConditionIndex(m.queryFields[queryFieldSortCondition].value)
	next := current + delta
	if next < 0 {
		next = len(querySortKeyConditions) - 1
	}
	if next >= len(querySortKeyConditions) {
		next = 0
	}

	m.queryFields[queryFieldSortCondition].value = querySortKeyConditions[next]
	if m.queryFields[queryFieldSortCondition].value != "between" && len(m.queryFields) > queryFieldSortValueEnd {
		m.queryFields[queryFieldSortValueEnd].value = ""
	}
	if m.queryFocus > m.queryInputCount() {
		m.queryFocus = m.queryInputCount()
	}
}

func (m *Model) openSelectedResultDetail() error {
	if len(m.resultItems) == 0 {
		return errors.New("no rows available")
	}

	if m.resultSelected < 0 {
		m.resultSelected = 0
	}
	if m.resultSelected >= len(m.resultItems) {
		m.resultSelected = len(m.resultItems) - 1
	}

	if err := m.transitionTo(viewStateDetail); err != nil {
		return err
	}
	m.detailSelected = 0
	m.detailScroll = 0
	m.jsonModalOpen = false
	m.jsonModalLines = nil
	m.jsonModalScroll = 0
	m.status = fmt.Sprintf("opened row %d detail", m.resultSelected+1)
	return nil
}

func (m Model) selectedResultItems() (map[string]interface{}, map[string]interface{}, bool) {
	if len(m.resultItems) == 0 {
		return nil, nil, false
	}

	index := m.resultSelected
	if index < 0 {
		index = 0
	}
	if index >= len(m.resultItems) {
		index = len(m.resultItems) - 1
	}

	display := m.resultItems[index]
	var raw map[string]interface{}
	if index < len(m.resultRawItems) && m.resultRawItems[index] != nil {
		raw = m.resultRawItems[index]
	} else {
		raw = display
	}

	return display, raw, true
}

func (m Model) selectedResultKeys() []string {
	_, rawItem, ok := m.selectedResultItems()
	if !ok {
		return nil
	}

	keys := make([]string, 0, len(rawItem))
	for key := range rawItem {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func (m Model) detailPageSize() int {
	_, contentHeight := m.stretchedPanelContentSize()
	pageSize := contentHeight - 6
	if pageSize < 3 {
		return 3
	}

	return pageSize
}

func (m *Model) clampDetailViewport() {
	keys := m.selectedResultKeys()
	if len(keys) == 0 {
		m.detailSelected = 0
		m.detailScroll = 0
		return
	}

	if m.detailSelected < 0 {
		m.detailSelected = 0
	}
	if m.detailSelected >= len(keys) {
		m.detailSelected = len(keys) - 1
	}

	pageSize := m.detailPageSize()
	maxScroll := len(keys) - pageSize
	if maxScroll < 0 {
		maxScroll = 0
	}

	if m.detailSelected < m.detailScroll {
		m.detailScroll = m.detailSelected
	}
	if m.detailSelected >= m.detailScroll+pageSize {
		m.detailScroll = m.detailSelected - pageSize + 1
	}

	if m.detailScroll < 0 {
		m.detailScroll = 0
	}
	if m.detailScroll > maxScroll {
		m.detailScroll = maxScroll
	}
}

func (m *Model) moveDetailSelection(delta int) {
	keys := m.selectedResultKeys()
	if len(keys) == 0 {
		m.detailSelected = 0
		m.detailScroll = 0
		m.status = "no fields available"
		return
	}

	m.detailSelected += delta
	if m.detailSelected < 0 {
		m.detailSelected = 0
	}
	if m.detailSelected >= len(keys) {
		m.detailSelected = len(keys) - 1
	}

	m.clampDetailViewport()
	m.status = fmt.Sprintf("field %d of %d (%s)", m.detailSelected+1, len(keys), keys[m.detailSelected])
}

func (m *Model) openJSONModal() error {
	_, rawItem, ok := m.selectedResultItems()
	if !ok {
		return errors.New("no rows available")
	}

	payload, err := json.MarshalIndent(rawItem, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to render JSON item: %w", err)
	}

	m.jsonModalLines = strings.Split(string(payload), "\n")
	if len(m.jsonModalLines) == 0 {
		m.jsonModalLines = []string{"{}"}
	}
	m.jsonModalScroll = 0
	m.jsonModalOpen = true
	m.jsonSearchMode = false
	m.jsonSearchInput = ""
	m.jsonSearchQuery = ""
	m.jsonSearchMatches = nil
	m.jsonSearchCurrent = -1
	m.clampJSONModalViewport()
	m.status = fmt.Sprintf("opened JSON modal for row %d", m.resultSelected+1)

	return nil
}

func (m Model) jsonModalPageSize() int {
	pageSize := m.height - 16
	if pageSize < 4 {
		return 4
	}

	return pageSize
}

func (m *Model) clampJSONModalViewport() {
	if len(m.jsonModalLines) == 0 {
		m.jsonModalScroll = 0
		return
	}

	maxScroll := len(m.jsonModalLines) - m.jsonModalPageSize()
	if maxScroll < 0 {
		maxScroll = 0
	}

	if m.jsonModalScroll < 0 {
		m.jsonModalScroll = 0
	}
	if m.jsonModalScroll > maxScroll {
		m.jsonModalScroll = maxScroll
	}
}

func (m *Model) moveJSONModalScroll(delta int) {
	if len(m.jsonModalLines) == 0 {
		m.status = "modal has no content"
		m.jsonModalScroll = 0
		return
	}

	m.jsonModalScroll += delta
	m.clampJSONModalViewport()

	end := m.jsonModalScroll + m.jsonModalPageSize()
	if end > len(m.jsonModalLines) {
		end = len(m.jsonModalLines)
	}

	m.status = fmt.Sprintf("JSON lines %d-%d of %d", m.jsonModalScroll+1, end, len(m.jsonModalLines))
}

func (m Model) updateJSONSearchInput(msg tea.KeyMsg) Model {
	switch msg.Type {
	case tea.KeyEsc:
		m.jsonSearchMode = false
		m.status = "closed JSON search"
		return m
	case tea.KeyEnter:
		return m.executeJSONSearch()
	case tea.KeyBackspace, tea.KeyDelete:
		runes := []rune(m.jsonSearchInput)
		if len(runes) > 0 {
			m.jsonSearchInput = string(runes[:len(runes)-1])
		}
		m.status = fmt.Sprintf("search JSON: %s", m.jsonSearchInput)
		return m
	case tea.KeyRunes:
		m.jsonSearchInput += string(msg.Runes)
		m.status = fmt.Sprintf("search JSON: %s", m.jsonSearchInput)
		return m
	default:
		return m
	}
}

func (m Model) executeJSONSearch() Model {
	query := strings.TrimSpace(m.jsonSearchInput)
	if query == "" {
		m.status = "search query is empty"
		m.jsonSearchMode = false
		return m
	}

	loweredQuery := strings.ToLower(query)
	matches := make([]int, 0)
	for idx, line := range m.jsonModalLines {
		if strings.Contains(strings.ToLower(line), loweredQuery) {
			matches = append(matches, idx)
		}
	}

	m.jsonSearchQuery = query
	m.jsonSearchInput = query
	m.jsonSearchMatches = matches
	m.jsonSearchCurrent = -1
	m.jsonSearchMode = false

	if len(matches) == 0 {
		m.status = fmt.Sprintf("no JSON matches for %q", query)
		return m
	}

	current := 0
	for idx, matchLine := range matches {
		if matchLine >= m.jsonModalScroll {
			current = idx
			break
		}
	}
	m.jsonSearchCurrent = current
	m.focusJSONMatch(current)

	targetLine := matches[current]
	m.status = fmt.Sprintf("JSON match %d/%d at line %d", current+1, len(matches), targetLine+1)
	return m
}

func (m *Model) focusJSONMatch(matchIndex int) {
	if len(m.jsonSearchMatches) == 0 {
		return
	}
	if matchIndex < 0 {
		matchIndex = 0
	}
	if matchIndex >= len(m.jsonSearchMatches) {
		matchIndex = len(m.jsonSearchMatches) - 1
	}

	line := m.jsonSearchMatches[matchIndex]
	scroll := line - (m.jsonModalPageSize() / 2)
	if scroll < 0 {
		scroll = 0
	}
	m.jsonModalScroll = scroll
	m.clampJSONModalViewport()
}

func sanitizeFileToken(value string) string {
	trimmed := strings.TrimSpace(strings.ToLower(value))
	if trimmed == "" {
		return "item"
	}

	var builder strings.Builder
	for _, r := range trimmed {
		switch {
		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'):
			builder.WriteRune(r)
		case r == '-' || r == '_' || r == '.':
			builder.WriteRune(r)
		default:
			builder.WriteRune('-')
		}
	}

	result := strings.Trim(builder.String(), "-._")
	if result == "" {
		return "item"
	}

	return result
}

func (m Model) saveSelectedResultRawItemToCurrentDir() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("resolve current directory: %w", err)
	}

	return m.saveSelectedResultRawItemToDir(dir)
}

func (m Model) saveSelectedResultRawItemToDir(dir string) (string, error) {
	_, rawItem, ok := m.selectedResultItems()
	if !ok {
		return "", errors.New("no rows available")
	}

	payload, err := json.MarshalIndent(rawItem, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal item JSON: %w", err)
	}

	tableToken := sanitizeFileToken(m.activeTable)
	if tableToken == "" {
		tableToken = "table"
	}

	filename := fmt.Sprintf("%s-row-%d-%s.json", tableToken, m.resultSelected+1, time.Now().Format("20060102-150405"))
	path := filepath.Join(dir, filename)
	if err := os.WriteFile(path, payload, 0o600); err != nil {
		return "", fmt.Errorf("write item JSON to %s: %w", path, err)
	}

	return path, nil
}

func (m Model) queryFocusLabel() string {
	if m.queryFocus >= m.queryInputCount() {
		return "Run Query"
	}

	return m.queryFields[m.queryFocus].label
}

func (m *Model) updateQueryInput(msg tea.KeyMsg) bool {
	if m.queryFocus < 0 || m.queryFocus >= m.queryInputCount() {
		return false
	}
	if m.queryFocus == queryFieldSortCondition {
		return false
	}

	field := &m.queryFields[m.queryFocus]
	switch msg.Type {
	case tea.KeyRunes:
		field.value += string(msg.Runes)
		return true
	case tea.KeyBackspace, tea.KeyDelete:
		if field.value == "" {
			return true
		}
		runes := []rune(field.value)
		field.value = string(runes[:len(runes)-1])
		return true
	default:
		return false
	}
}

func (m Model) buildQueryRequest() (queryRequest, error) {
	if m.activeTable == "" {
		return queryRequest{}, errors.New("no table selected")
	}
	target, ok := m.currentQueryTarget()
	if !ok {
		return queryRequest{}, errors.New("query form is not initialized")
	}

	request := queryRequest{
		tableName:      m.activeTable,
		partitionKey:   strings.TrimSpace(target.partitionKey),
		partitionValue: strings.TrimSpace(m.queryFields[queryFieldPartitionValue].value),
		sortKey:        strings.TrimSpace(target.sortKey),
		condition:      strings.TrimSpace(m.queryFields[queryFieldSortCondition].value),
		sortValue:      strings.TrimSpace(m.queryFields[queryFieldSortValue].value),
		sortValueEnd:   strings.TrimSpace(m.queryFields[queryFieldSortValueEnd].value),
		indexName:      strings.TrimSpace(target.indexName),
	}

	if request.partitionKey == "" {
		return queryRequest{}, errors.New("partition key name is required")
	}
	if request.partitionValue == "" {
		return queryRequest{}, errors.New("partition key value is required")
	}

	if request.sortKey == "" {
		request.sortValue = ""
		request.sortValueEnd = ""
		request.condition = querySortKeyConditions[0]
		return request, nil
	}

	if strings.TrimSpace(request.sortValue) == "" {
		request.sortValue = ""
		request.sortValueEnd = ""
		request.condition = querySortKeyConditions[0]
		request.sortKey = ""
		return request, nil
	}
	if !isValidSortCondition(request.condition) {
		return queryRequest{}, fmt.Errorf("invalid sort key condition %q", request.condition)
	}
	if request.condition == "between" {
		if request.sortValueEnd == "" {
			return queryRequest{}, errors.New("sort key end value is required for between")
		}
		return request, nil
	}

	request.sortValueEnd = ""

	return request, nil
}

func runQueryCmd(client *aws.Client, request queryRequest, requestID uint64) tea.Cmd {
	return func() tea.Msg {
		if client == nil {
			return queryErrorMsg{requestID: requestID, tableName: request.tableName, err: errors.New("aws client is nil")}
		}

		result, err := client.QueryContext(
			context.Background(),
			request.tableName,
			request.partitionKey,
			request.partitionValue,
			request.sortKey,
			request.sortValue,
			request.sortValueEnd,
			request.condition,
			request.indexName,
			nil,
		)
		if err != nil {
			return queryErrorMsg{requestID: requestID, tableName: request.tableName, err: err}
		}

		return querySuccessMsg{requestID: requestID, tableName: request.tableName, result: result}
	}
}

func runScanCmd(client *aws.Client, tableName string, requestID uint64) tea.Cmd {
	return func() tea.Msg {
		if client == nil {
			return scanErrorMsg{requestID: requestID, tableName: tableName, err: errors.New("aws client is nil")}
		}

		result, err := client.ScanContext(context.Background(), tableName, nil)
		if err != nil {
			return scanErrorMsg{requestID: requestID, tableName: tableName, err: err}
		}

		return scanSuccessMsg{requestID: requestID, tableName: tableName, result: result}
	}
}

func defaultQueryFields() []queryField {
	return []queryField{
		{label: "Partition Key Value", placeholder: "required value", required: true},
		{label: "Sort Key Condition", value: querySortKeyConditions[0], placeholder: "=, begins_with, <, <=, >, >=, between"},
		{label: "Sort Key Value", placeholder: "optional"},
		{label: "Sort Key Value (End)", placeholder: "required when condition is between"},
	}
}
