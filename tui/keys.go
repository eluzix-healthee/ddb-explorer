package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
)

type KeyMap struct {
	Global    GlobalKeyMap
	TableList TableListKeyMap
	Query     QueryKeyMap
	Scan      ScanKeyMap
}

type GlobalKeyMap struct {
	Quit key.Binding
	Back key.Binding
	Help key.Binding
}

type TableListKeyMap struct {
	MoveUp   key.Binding
	MoveDown key.Binding
	Filter   key.Binding
	Open     key.Binding
	Refresh  key.Binding
}

type QueryKeyMap struct {
	NextField  key.Binding
	PrevField  key.Binding
	Submit     key.Binding
	SwitchScan key.Binding
}

type ScanKeyMap struct {
	Run      key.Binding
	NextPage key.Binding
	PrevPage key.Binding
}

func defaultKeyMap() KeyMap {
	return KeyMap{
		Global: GlobalKeyMap{
			Quit: key.NewBinding(
				key.WithKeys("q", "ctrl+c"),
				key.WithHelp("q", "quit"),
			),
			Back: key.NewBinding(
				key.WithKeys("esc"),
				key.WithHelp("esc", "back"),
			),
			Help: key.NewBinding(
				key.WithKeys("ctrl+h"),
				key.WithHelp("ctrl+h", "toggle help"),
			),
		},
		TableList: TableListKeyMap{
			MoveUp: key.NewBinding(
				key.WithKeys("up", "k"),
				key.WithHelp("up/k", "move up"),
			),
			MoveDown: key.NewBinding(
				key.WithKeys("down", "j"),
				key.WithHelp("down/j", "move down"),
			),
			Filter: key.NewBinding(
				key.WithKeys("/"),
				key.WithHelp("/", "filter"),
			),
			Open: key.NewBinding(
				key.WithKeys("enter"),
				key.WithHelp("enter", "open query/scan"),
			),
			Refresh: key.NewBinding(
				key.WithKeys("r"),
				key.WithHelp("r", "reload tables"),
			),
		},
		Query: QueryKeyMap{
			NextField: key.NewBinding(
				key.WithKeys("tab"),
				key.WithHelp("tab", "next field"),
			),
			PrevField: key.NewBinding(
				key.WithKeys("shift+tab"),
				key.WithHelp("shift+tab", "prev field"),
			),
			Submit: key.NewBinding(
				key.WithKeys("enter"),
				key.WithHelp("enter", "run query"),
			),
			SwitchScan: key.NewBinding(
				key.WithKeys("ctrl+s"),
				key.WithHelp("ctrl+s", "switch to scan"),
			),
		},
		Scan: ScanKeyMap{
			Run: key.NewBinding(
				key.WithKeys("enter"),
				key.WithHelp("enter", "run scan"),
			),
			NextPage: key.NewBinding(
				key.WithKeys("n"),
				key.WithHelp("n", "next page"),
			),
			PrevPage: key.NewBinding(
				key.WithKeys("p"),
				key.WithHelp("p", "previous page"),
			),
		},
	}
}

func (k KeyMap) HelpText(state viewState) string {
	bindings := make([]key.Binding, 0, 8)
	bindings = append(bindings, k.globalBindings()...)
	bindings = append(bindings, k.stateBindings(state)...)

	parts := make([]string, 0, len(bindings))
	for _, binding := range bindings {
		help := binding.Help()
		if help.Key == "" || help.Desc == "" {
			continue
		}
		parts = append(parts, help.Key+": "+help.Desc)
	}

	return strings.Join(parts, " | ")
}

func (k KeyMap) globalBindings() []key.Binding {
	return []key.Binding{k.Global.Quit, k.Global.Back, k.Global.Help}
}

func (k KeyMap) stateBindings(state viewState) []key.Binding {
	switch state {
	case viewStateTables:
		return []key.Binding{
			k.TableList.MoveUp,
			k.TableList.MoveDown,
			k.TableList.Filter,
			k.TableList.Open,
			k.TableList.Refresh,
		}
	case viewStateQuery:
		return []key.Binding{
			k.Query.NextField,
			k.Query.PrevField,
			k.Query.Submit,
			k.Query.SwitchScan,
		}
	case viewStateScan:
		return []key.Binding{
			k.Scan.Run,
			k.Scan.NextPage,
			k.Scan.PrevPage,
		}
	default:
		return nil
	}
}
