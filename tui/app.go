package tui

import (
	"errors"
	"fmt"
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
	viewStateError   viewState = "error"
)

type Model struct {
	profile string
	client  *aws.Client
	width   int
	height  int
	state   viewState
	tables  []aws.TableInfo
	status  string
	err     error

	tableFilter       string
	filterInputActive bool
	selectedTable     int
	activeTable       string

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
		profile: profile,
		client:  client,
		width:   80,
		height:  24,
		state:   viewStateLoading,
		status:  "loading DynamoDB tables",
		spinner: spin,
		keys:    defaultKeyMap(),
		theme:   theme,
	}
}

func NewProgram(model Model) *tea.Program {
	return tea.NewProgram(model, tea.WithAltScreen())
}

func (m Model) Init() tea.Cmd {
	return m.loadTablesWithSpinnerCmd()
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
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
		}
	case tableLoadSuccessMsg:
		m.tables = msg.tables
		m.state = viewStateTables
		m.activeTable = ""
		m.filterInputActive = false
		m.selectedTable = 0
		visible := len(m.filteredTables())
		m.status = fmt.Sprintf("loaded %d tables (%d visible) with profile %s", len(msg.tables), visible, m.profile)
		m.err = nil
		return m, nil
	case tableLoadErrorMsg:
		m.state = viewStateError
		m.status = "failed to load tables"
		m.err = normalizeError(msg.err, "table load failed")
		return m, nil
	case querySuccessMsg:
		m.status = fmt.Sprintf("query returned %d items from %s", len(msg.result.Items), msg.tableName)
		m.err = nil
		return m, nil
	case queryErrorMsg:
		m.status = fmt.Sprintf("query failed for %s", msg.tableName)
		m.err = normalizeError(msg.err, "query failed")
		return m, nil
	case scanSuccessMsg:
		m.status = fmt.Sprintf("scan returned %d items from %s", len(msg.result.Items), msg.tableName)
		m.err = nil
		return m, nil
	case scanErrorMsg:
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
	case viewStateError:
		content = m.errorView()
	default:
		content = m.theme.Error.Render("invalid app state")
	}

	if m.showHelp {
		content = m.withHelp(content)
	}

	return m.renderChrome(content)
}

func (m Model) loadTablesWithSpinnerCmd() tea.Cmd {
	return tea.Batch(m.spinner.Tick, loadTablesCmd(m.client))
}

func loadTablesCmd(client *aws.Client) tea.Cmd {
	return func() tea.Msg {
		if client == nil {
			return tableLoadErrorMsg{err: errors.New("aws client is nil")}
		}

		tables, err := client.ListTables()
		if err != nil {
			return tableLoadErrorMsg{err: err}
		}

		return tableLoadSuccessMsg{tables: tables}
	}
}

func normalizeError(err error, fallback string) error {
	if err != nil {
		return err
	}

	return errors.New(fallback)
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
		m.state = viewStateTables
		m.status = "returned to table list"
		return m, nil
	case viewStateError:
		if len(m.tables) > 0 {
			m.state = viewStateTables
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
		m.state = viewStateLoading
		m.status = "refreshing table list"
		m.err = nil
		m.filterInputActive = false
		return m, m.loadTablesWithSpinnerCmd()
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

		m.state = viewStateQuery
		m.activeTable = table.Name
		m.filterInputActive = false
		m.status = fmt.Sprintf("opened query flow for %s", table.Name)
		return m, nil
	}

	return m, nil
}

func (m Model) updateQueryKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if key.Matches(msg, m.keys.Query.SwitchScan) {
		m.state = viewStateScan
		m.status = fmt.Sprintf("switched to scan flow for %s", m.activeTable)
		return m, nil
	}
	if key.Matches(msg, m.keys.Query.Submit) {
		m.status = fmt.Sprintf("query form for %s arrives in US-007", m.activeTable)
		return m, nil
	}
	if key.Matches(msg, m.keys.Query.NextField) || key.Matches(msg, m.keys.Query.PrevField) {
		m.status = "field navigation will be wired in US-007"
		return m, nil
	}

	return m, nil
}

func (m Model) updateScanKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if key.Matches(msg, m.keys.Scan.Run) {
		m.status = fmt.Sprintf("scan form for %s arrives in US-007", m.activeTable)
		return m, nil
	}
	if key.Matches(msg, m.keys.Scan.NextPage) || key.Matches(msg, m.keys.Scan.PrevPage) {
		m.status = "scan pagination arrives in US-008"
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
