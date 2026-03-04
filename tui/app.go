package tui

import (
	"errors"
	"fmt"

	"ddb-explorer/aws"

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
	status  string
	err     error

	showHelp bool
	spinner  spinner.Model
	keys     KeyMap
	styles   Styles
}

func NewModel(profile string, client *aws.Client) Model {
	spin := spinner.New()
	spin.Spinner = spinner.Dot
	spin.Style = defaultStyles().Highlight

	return Model{
		profile: profile,
		client:  client,
		width:   80,
		height:  24,
		state:   viewStateLoading,
		status:  "connecting to AWS",
		spinner: spin,
		keys:    defaultKeyMap(),
		styles:  defaultStyles(),
	}
}

func NewProgram(model Model) *tea.Program {
	return tea.NewProgram(model, tea.WithAltScreen())
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, testConnectionCmd(m.client))
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case tea.KeyMsg:
		if key.Matches(msg, m.keys.Quit) {
			return m, tea.Quit
		}
		if key.Matches(msg, m.keys.Help) {
			m.showHelp = !m.showHelp
			return m, nil
		}
		if key.Matches(msg, m.keys.Back) {
			m.status = "already at root view"
			return m, nil
		}
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	case connectionReadyMsg:
		m.state = viewStateTables
		m.status = fmt.Sprintf("connected with profile %s", m.profile)
		m.err = nil
		return m, nil
	case connectionFailedMsg:
		m.state = viewStateError
		m.status = "failed to connect"
		m.err = msg.err
		return m, nil
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
		content = m.styles.Error.Render("invalid app state")
	}

	if m.showHelp {
		content = m.withHelp(content)
	}

	return m.renderChrome(content)
}

func testConnectionCmd(client *aws.Client) tea.Cmd {
	return func() tea.Msg {
		if client == nil {
			return connectionFailedMsg{err: errors.New("aws client is nil")}
		}
		if err := client.TestConnection(); err != nil {
			return connectionFailedMsg{err: err}
		}
		return connectionReadyMsg{}
	}
}
