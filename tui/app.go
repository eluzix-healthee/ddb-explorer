package tui

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

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
	queryFieldPartitionKey = iota
	queryFieldPartitionValue
	queryFieldSortKey
	queryFieldSortValue
	queryFieldIndexName
)

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
	sortValue      string
	indexName      string
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

	tableFilter       string
	filterInputActive bool
	selectedTable     int
	activeTable       string
	queryFields       []queryField
	queryFocus        int
	resultItems       []map[string]interface{}
	resultRawItems    []map[string]interface{}
	resultColumns     []string
	resultSelected    int
	resultPage        int
	resultOrigin      viewState
	resultHasMore     bool
	detailSelected    int
	detailScroll      int
	jsonModalOpen     bool
	jsonModalLines    []string
	jsonModalScroll   int

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
		queryFields: []queryField{
			{label: "Partition Key Name", placeholder: "e.g. account_id", required: true},
			{label: "Partition Key Value", placeholder: "required value", required: true},
			{label: "Sort Key Name", placeholder: "optional"},
			{label: "Sort Key Value", placeholder: "optional"},
			{label: "Index Name", placeholder: "optional GSI/LSI"},
		},
		spinner: spin,
		keys:    defaultKeyMap(),
		theme:   theme,
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
		if key.Matches(msg, m.keys.Global.Help) {
			m.showHelp = !m.showHelp
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
		if m.queryFocus < len(m.queryFields) {
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

	if m.queryFocus >= len(m.queryFields) {
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
		if key.Matches(msg, m.keys.Detail.OpenJSON) {
			m.jsonModalOpen = false
			m.status = "closed JSON modal"
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
	m.queryFields = []queryField{
		{
			label:       "Partition Key Name",
			value:       table.PartitionKey,
			placeholder: "e.g. account_id",
			required:    true,
		},
		{
			label:       "Partition Key Value",
			placeholder: "required value",
			required:    true,
		},
		{
			label:       "Sort Key Name",
			value:       table.SortKey,
			placeholder: "optional",
		},
		{
			label:       "Sort Key Value",
			placeholder: "optional",
		},
		{
			label:       "Index Name",
			placeholder: "optional GSI/LSI",
		},
	}
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
	m.resultColumns = discoverResultColumns(result.Items)
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

func (m Model) resultPageSize() int {
	pageSize := m.height - 18
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
	max := len(m.queryFields)
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
	item, _, ok := m.selectedResultItems()
	if !ok {
		return nil
	}

	keys := make([]string, 0, len(item))
	for key := range item {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func (m Model) detailPageSize() int {
	pageSize := m.height - 18
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

func (m Model) queryFocusLabel() string {
	if m.queryFocus >= len(m.queryFields) {
		return "Run Query"
	}

	return m.queryFields[m.queryFocus].label
}

func (m *Model) updateQueryInput(msg tea.KeyMsg) bool {
	if m.queryFocus < 0 || m.queryFocus >= len(m.queryFields) {
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

	if len(m.queryFields) <= queryFieldIndexName {
		return queryRequest{}, errors.New("query form is not initialized")
	}

	request := queryRequest{
		tableName:      m.activeTable,
		partitionKey:   strings.TrimSpace(m.queryFields[queryFieldPartitionKey].value),
		partitionValue: strings.TrimSpace(m.queryFields[queryFieldPartitionValue].value),
		sortKey:        strings.TrimSpace(m.queryFields[queryFieldSortKey].value),
		sortValue:      strings.TrimSpace(m.queryFields[queryFieldSortValue].value),
		indexName:      strings.TrimSpace(m.queryFields[queryFieldIndexName].value),
	}

	if request.partitionKey == "" {
		return queryRequest{}, errors.New("partition key name is required")
	}
	if request.partitionValue == "" {
		return queryRequest{}, errors.New("partition key value is required")
	}

	hasSortKey := request.sortKey != ""
	hasSortValue := request.sortValue != ""
	if hasSortKey != hasSortValue {
		return queryRequest{}, errors.New("sort key name and value must both be set or both be empty")
	}

	return request, nil
}

func runQueryCmd(client *aws.Client, request queryRequest, requestID uint64) tea.Cmd {
	return func() tea.Msg {
		if client == nil {
			return queryErrorMsg{requestID: requestID, tableName: request.tableName, err: errors.New("aws client is nil")}
		}

		result, err := client.Query(
			request.tableName,
			request.partitionKey,
			request.partitionValue,
			request.sortKey,
			request.sortValue,
			"=",
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

		result, err := client.Scan(tableName, nil)
		if err != nil {
			return scanErrorMsg{requestID: requestID, tableName: tableName, err: err}
		}

		return scanSuccessMsg{requestID: requestID, tableName: tableName, result: result}
	}
}
