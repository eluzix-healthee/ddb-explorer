package tui

import "github.com/charmbracelet/lipgloss"

type Styles struct {
	Title       lipgloss.Style
	Body        lipgloss.Style
	Hint        lipgloss.Style
	Error       lipgloss.Style
	Success     lipgloss.Style
	Highlight   lipgloss.Style
	Panel       lipgloss.Style
	Help        lipgloss.Style
	ChromeTitle lipgloss.Style
	ChromeHint  lipgloss.Style
}

func defaultStyles() Styles {
	baseFg := lipgloss.Color("252")
	mutedFg := lipgloss.Color("246")
	highlight := lipgloss.Color("214")
	errorColor := lipgloss.Color("203")
	successColor := lipgloss.Color("42")
	panelBg := lipgloss.Color("236")

	return Styles{
		Title: lipgloss.NewStyle().
			Bold(true).
			Foreground(highlight),
		Body: lipgloss.NewStyle().
			Foreground(baseFg),
		Hint: lipgloss.NewStyle().
			Foreground(mutedFg),
		Error: lipgloss.NewStyle().
			Bold(true).
			Foreground(errorColor),
		Success: lipgloss.NewStyle().
			Bold(true).
			Foreground(successColor),
		Highlight: lipgloss.NewStyle().
			Foreground(highlight),
		Panel: lipgloss.NewStyle().
			Background(panelBg).
			Padding(1, 2).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(highlight),
		Help: lipgloss.NewStyle().
			Foreground(mutedFg).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(mutedFg).
			Padding(0, 1),
		ChromeTitle: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("0")).
			Background(highlight).
			Padding(0, 1),
		ChromeHint: lipgloss.NewStyle().
			Foreground(baseFg).
			Background(lipgloss.Color("238")).
			Padding(0, 1),
	}
}
