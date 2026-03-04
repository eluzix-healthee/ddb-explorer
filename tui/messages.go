package tui

type connectionReadyMsg struct{}

type connectionFailedMsg struct {
	err error
}
