package styles

import "github.com/charmbracelet/lipgloss"

type Palette struct {
	BaseForeground  lipgloss.Color
	MutedForeground lipgloss.Color
	Highlight       lipgloss.Color
	Error           lipgloss.Color
	Success         lipgloss.Color
	PanelBackground lipgloss.Color
	ChromeMuted     lipgloss.Color
}

func DefaultPalette() Palette {
	return Palette{
		BaseForeground:  lipgloss.Color("252"),
		MutedForeground: lipgloss.Color("246"),
		Highlight:       lipgloss.Color("214"),
		Error:           lipgloss.Color("203"),
		Success:         lipgloss.Color("42"),
		PanelBackground: lipgloss.Color("236"),
		ChromeMuted:     lipgloss.Color("238"),
	}
}

type Theme struct {
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

func Default() Theme {
	palette := DefaultPalette()

	baseText := lipgloss.NewStyle().Foreground(palette.BaseForeground)
	mutedText := lipgloss.NewStyle().Foreground(palette.MutedForeground)
	highlightText := lipgloss.NewStyle().Foreground(palette.Highlight)

	return Theme{
		Title: highlightText.Bold(true),
		Body:  baseText,
		Hint:  mutedText,
		Error: lipgloss.NewStyle().
			Bold(true).
			Foreground(palette.Error),
		Success: lipgloss.NewStyle().
			Bold(true).
			Foreground(palette.Success),
		Highlight: highlightText,
		Panel: lipgloss.NewStyle().
			Background(palette.PanelBackground).
			Padding(1, 2).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(palette.Highlight),
		Help: lipgloss.NewStyle().
			Foreground(palette.MutedForeground).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(palette.MutedForeground).
			Padding(0, 1),
		ChromeTitle: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("0")).
			Background(palette.Highlight).
			Padding(0, 1),
		ChromeHint: lipgloss.NewStyle().
			Foreground(palette.BaseForeground).
			Background(palette.ChromeMuted).
			Padding(0, 1),
	}
}
