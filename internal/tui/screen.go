package tui

import (
	tea "github.com/charmbracelet/bubbletea"
)

// screenKind identifies which screen is active. Used for navigation and for
// the app to know which screen to instantiate when going back.
type screenKind int

const (
	screenMenu screenKind = iota
	screenInstall
	screenUpdate
	screenRemove
	screenInstalled
	screenSync
	screenSettings
	screenImport
)

// screen is implemented by every TUI screen. A screen is a self-contained
// bubbletea model that owns its list/state; the app delegates Update and View
// to the active screen and listens for navigation messages to switch screens.
type screen interface {
	tea.Model
	// title returns the screen heading shown in the header bar.
	title() string
	// help returns the keybinding hint shown in the footer.
	help() string
	// enter is called whenever the app makes this screen active, giving it
	// a chance to refresh its data (e.g. re-read the installed database).
	enter()
}

// navMsg switches the app to a different screen.
type navMsg struct{ to screenKind }

// nav returns a command that navigates to a screen. Wrapped in a Cmd because
// screens emit it from Update, which only returns tea.Cmd.
func nav(to screenKind) tea.Cmd {
	return func() tea.Msg { return navMsg{to: to} }
}

// backMsg pops to the previous screen (the menu). Screens send this on Esc.
type backMsg struct{}

// back is the command form of backMsg.
func back() tea.Cmd {
	return func() tea.Msg { return backMsg{} }
}

// quitMsg requests the app to terminate.
type quitMsg struct{}

// quit is the command form of quitMsg.
func quit() tea.Cmd {
	return func() tea.Msg { return quitMsg{} }
}
