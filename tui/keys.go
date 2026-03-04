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
	Results   ResultsKeyMap
	Detail    DetailKeyMap
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
	NextField   key.Binding
	PrevField   key.Binding
	NextOption  key.Binding
	PrevOption  key.Binding
	CycleTarget key.Binding
	Submit      key.Binding
	SwitchScan  key.Binding
}

type ScanKeyMap struct {
	Run key.Binding
}

type ResultsKeyMap struct {
	MoveUp   key.Binding
	MoveDown key.Binding
	NextPage key.Binding
	PrevPage key.Binding
	Open     key.Binding
}

type DetailKeyMap struct {
	MoveUp   key.Binding
	MoveDown key.Binding
	NextPage key.Binding
	PrevPage key.Binding
	OpenJSON key.Binding
	SaveItem key.Binding
	Search   key.Binding
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
			NextOption: key.NewBinding(
				key.WithKeys("right", "]"),
				key.WithHelp("right/]", "next option"),
			),
			PrevOption: key.NewBinding(
				key.WithKeys("left", "["),
				key.WithHelp("left/[", "prev option"),
			),
			CycleTarget: key.NewBinding(
				key.WithKeys("ctrl+g"),
				key.WithHelp("ctrl+g", "next key source"),
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
		},
		Results: ResultsKeyMap{
			MoveUp: key.NewBinding(
				key.WithKeys("up", "k"),
				key.WithHelp("up/k", "move row up"),
			),
			MoveDown: key.NewBinding(
				key.WithKeys("down", "j"),
				key.WithHelp("down/j", "move row down"),
			),
			NextPage: key.NewBinding(
				key.WithKeys("n", "right", "pgdown"),
				key.WithHelp("n", "next page"),
			),
			PrevPage: key.NewBinding(
				key.WithKeys("p", "left", "pgup"),
				key.WithHelp("p", "previous page"),
			),
			Open: key.NewBinding(
				key.WithKeys("enter"),
				key.WithHelp("enter", "open row detail"),
			),
		},
		Detail: DetailKeyMap{
			MoveUp: key.NewBinding(
				key.WithKeys("up", "k"),
				key.WithHelp("up/k", "move detail"),
			),
			MoveDown: key.NewBinding(
				key.WithKeys("down", "j"),
				key.WithHelp("down/j", "move detail"),
			),
			NextPage: key.NewBinding(
				key.WithKeys("n", "right", "pgdown"),
				key.WithHelp("n", "next page"),
			),
			PrevPage: key.NewBinding(
				key.WithKeys("p", "left", "pgup"),
				key.WithHelp("p", "previous page"),
			),
			OpenJSON: key.NewBinding(
				key.WithKeys("enter"),
				key.WithHelp("enter", "toggle JSON modal"),
			),
			SaveItem: key.NewBinding(
				key.WithKeys("s"),
				key.WithHelp("s", "save item JSON"),
			),
			Search: key.NewBinding(
				key.WithKeys("/"),
				key.WithHelp("/", "search JSON"),
			),
		},
	}
}

func (k KeyMap) HelpText(state viewState) string {
	return strings.Join(k.HelpLines(state), " | ")
}

func (k KeyMap) HelpLines(state viewState) []string {
	if state == viewStateQuery {
		return []string{
			"q: quit",
			"esc: back",
			"ctrl+h: toggle help",
			"ctrl+g: next key source",
			"enter: run query",
			"ctrl+s: switch to scan",
			"[ / ]: sort cond",
			"tab: next field",
			"shift+tab: prev field",
		}
	}

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

	return parts
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
			k.Query.NextOption,
			k.Query.PrevOption,
			k.Query.CycleTarget,
			k.Query.Submit,
			k.Query.SwitchScan,
		}
	case viewStateScan:
		return []key.Binding{
			k.Scan.Run,
		}
	case viewStateResults:
		return []key.Binding{
			k.Results.MoveUp,
			k.Results.MoveDown,
			k.Results.NextPage,
			k.Results.PrevPage,
			k.Results.Open,
		}
	case viewStateDetail:
		return []key.Binding{
			k.Detail.MoveUp,
			k.Detail.MoveDown,
			k.Detail.NextPage,
			k.Detail.PrevPage,
			k.Detail.OpenJSON,
			k.Detail.SaveItem,
			k.Detail.Search,
		}
	default:
		return nil
	}
}
