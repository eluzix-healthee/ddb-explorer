package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
)

func TestLoadingViewShowsSpinnerContext(t *testing.T) {
	m := NewModel("staging", nil)

	view := m.loadingView()
	if !strings.Contains(view, "Connecting to DynamoDB") {
		t.Fatalf("loading view missing spinner text: %q", view)
	}
	if !strings.Contains(view, "Loading table metadata") {
		t.Fatalf("loading view missing context text: %q", view)
	}
	if !strings.Contains(view, "Profile: staging") {
		t.Fatalf("loading view missing profile context: %q", view)
	}
}

func TestSpinnerTickIgnoredOutsideLoadingState(t *testing.T) {
	m := NewModel("dev", nil)
	m.state = viewStateTables

	next, cmd := m.Update(spinner.TickMsg{})
	if cmd != nil {
		t.Fatal("expected no spinner command when not in loading state")
	}

	updated, ok := next.(Model)
	if !ok {
		t.Fatalf("expected Model, got %T", next)
	}
	if updated.state != viewStateTables {
		t.Fatalf("state changed unexpectedly: got %q", updated.state)
	}
}

func TestLoadingViewRepositionsOnResize(t *testing.T) {
	m := NewModel("dev", nil)

	largeModel, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	large, ok := largeModel.(Model)
	if !ok {
		t.Fatalf("expected Model, got %T", largeModel)
	}
	largeView := large.View()
	if got := len(strings.Split(largeView, "\n")); got != 40 {
		t.Fatalf("expected 40 view lines after resize, got %d", got)
	}
	largeIndex := lineIndexContaining(largeView, "Loading table metadata")
	if largeIndex == -1 {
		t.Fatalf("missing loading context line in large render: %q", largeView)
	}

	smallModel, _ := large.Update(tea.WindowSizeMsg{Width: 80, Height: 20})
	small, ok := smallModel.(Model)
	if !ok {
		t.Fatalf("expected Model, got %T", smallModel)
	}
	smallView := small.View()
	if got := len(strings.Split(smallView, "\n")); got != 20 {
		t.Fatalf("expected 20 view lines after resize, got %d", got)
	}
	smallIndex := lineIndexContaining(smallView, "Loading table metadata")
	if smallIndex == -1 {
		t.Fatalf("missing loading context line in small render: %q", smallView)
	}

	if largeIndex <= smallIndex {
		t.Fatalf("expected loading content to move lower on taller viewport: large=%d small=%d", largeIndex, smallIndex)
	}
}

func lineIndexContaining(view string, needle string) int {
	lines := strings.Split(view, "\n")
	for idx, line := range lines {
		if strings.Contains(line, needle) {
			return idx
		}
	}

	return -1
}
