package tui

import (
	"errors"
	"fmt"

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
	return tea.Batch(m.spinner.Tick, loadTablesCmd(m.client))
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
			if m.state == viewStateError && len(m.tables) > 0 {
				m.state = viewStateTables
				m.status = "returned to table list"
				m.err = nil
				return m, nil
			}
			m.status = "already at root view"
			return m, nil
		}

		if m.state == viewStateTables {
			if key.Matches(msg, m.keys.TableList.Refresh) {
				m.state = viewStateLoading
				m.status = "refreshing table list"
				return m, loadTablesCmd(m.client)
			}
			if key.Matches(msg, m.keys.TableList.Open) {
				m.status = "query and scan forms are coming in US-007"
				return m, nil
			}
			if key.Matches(msg, m.keys.TableList.Filter) {
				m.status = "table filtering is coming in US-006"
				return m, nil
			}
			if key.Matches(msg, m.keys.TableList.MoveUp) || key.Matches(msg, m.keys.TableList.MoveDown) {
				m.status = "table navigation is coming in US-006"
				return m, nil
			}
		}
	case tableLoadSuccessMsg:
		m.tables = msg.tables
		m.state = viewStateTables
		m.status = fmt.Sprintf("loaded %d tables with profile %s", len(msg.tables), m.profile)
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
