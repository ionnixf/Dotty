package tui

import (
	"fmt"
	"strings"

	"github.com/ion/dotty/internal/config"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// settingsRow enumerates the editable settings in display order. Adding a new
// setting is a one-line change here plus a case in renderValue and adjust.
type settingsRow int

const (
	rowRepoDir settingsRow = iota
	rowAutoUpdate
	rowConfirmRemove
	rowTheme
	rowCount
)

// settingsScreen edits the user's Settings, persisting immediately on each
// change. The live settings live in deps.settings; we mutate through a local
// copy and write it back via SaveSettings + the deps pointer so every other
// screen picks up theme/repo-dir changes on the next render.
type settingsScreen struct {
	deps    *deps
	cursor  int
	local   config.Settings
	status  string
	err     error
	input   textinput.Model
	editing bool
}

func newSettingsScreen(d *deps) *settingsScreen {
	ti := textinput.New()
	ti.Placeholder = "Default repo directory"
	ti.CharLimit = 200
	ti.Width = 40
	return &settingsScreen{deps: d, input: ti}
}

func (s *settingsScreen) title() string { return "Settings" }
func (s *settingsScreen) help() string {
	if s.editing {
		return "enter confirm · esc cancel"
	}
	return "↑↓ navigate · enter/space toggle · backspace clear · s save · esc back"
}

// enter copies the current settings into the editable local buffer.
func (s *settingsScreen) enter() {
	if s.deps.settings != nil {
		s.local = *s.deps.settings
	}
	s.input.SetValue(s.local.RepoDirectory)
	s.editing = false
	s.status = ""
	s.err = nil
}

func (s *settingsScreen) setSize(w, h int) {}

func (s *settingsScreen) Init() tea.Cmd { return nil }

func (s *settingsScreen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if s.editing {
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch keyID(msg) {
			case "enter":
				s.local.RepoDirectory = strings.TrimSpace(s.input.Value())
				s.editing = false
				s.input.Blur()
				return s, s.save()
			case "esc":
				s.editing = false
				s.input.Blur()
				s.input.SetValue(s.local.RepoDirectory)
				return s, nil
			}
		}
		var cmd tea.Cmd
		s.input, cmd = s.input.Update(msg)
		return s, cmd
	}

	switch msg := msg.(type) {
	case settingsSavedMsg:
		if msg.err != nil {
			s.err = msg.err
			s.status = ""
			return s, nil
		}
		// Apply the persisted settings to the shared deps on the main
		// goroutine. This is the only place shared state is written, so there
		// is no race with View. Rebuilding the theme here also means every
		// screen (which holds the same *Theme pointer) picks up the new look
		// on its next render — no restart required.
		if s.deps.settings != nil {
			s.deps.updateSettings(msg.saved)
		}
		if s.deps.theme != nil && msg.saved.Theme != s.deps.theme.Name {
			*s.deps.theme = NewTheme(msg.saved.Theme)
		}
		s.err = nil
		s.status = "Saved."
		return s, nil
	case tea.KeyMsg:
		switch keyID(msg) {
		case "esc", "q":
			// Discard unsaved edits and return.
			s.status = ""
			s.err = nil
			return s, back()
		case "up", "k":
			if s.cursor > 0 {
				s.cursor--
			} else {
				s.cursor = int(rowCount) - 1
			}
		case "down", "j":
			if s.cursor < int(rowCount)-1 {
				s.cursor++
			} else {
				s.cursor = 0
			}
		case "enter", " ":
			if settingsRow(s.cursor) == rowRepoDir {
				s.editing = true
				s.input.Focus()
				s.input.SetValue(s.local.RepoDirectory)
				s.input.CursorEnd()
				return s, nil
			}
			s.adjust(s.cursor)
			return s, s.save()
		case "backspace", "delete":
			if settingsRow(s.cursor) == rowRepoDir {
				s.local.RepoDirectory = ""
				s.input.SetValue("")
				return s, s.save()
			}
		case "s":
			return s, s.save()
		}
	}
	return s, nil
}

// adjust applies the edit appropriate to the focused row: toggle for booleans,
// cycle for the theme. RepoDirectory is cleared via backspace in Update.
func (s *settingsScreen) adjust(row int) {
	switch settingsRow(row) {
	case rowAutoUpdate:
		s.local.AutoUpdate = !s.local.AutoUpdate
	case rowConfirmRemove:
		s.local.ConfirmRemove = !s.local.ConfirmRemove
	case rowTheme:
		if s.local.Theme == "dark" {
			s.local.Theme = "light"
		} else {
			s.local.Theme = "dark"
		}
	}
}

// save persists the edited settings to disk. It deliberately does NOT mutate
// the shared deps: a tea.Cmd runs on a background goroutine, while View reads
// deps.theme on the main goroutine, so writing shared state here would race
// and cause partial theme updates. Instead we return the saved settings in
// the message and apply them to deps in Update, which is single-threaded.
func (s *settingsScreen) save() tea.Cmd {
	settings := s.local
	paths := s.deps.paths
	return func() tea.Msg {
		if err := config.SaveSettings(paths, settings); err != nil {
			return settingsSavedMsg{err: err}
		}
		return settingsSavedMsg{saved: settings}
	}
}

func (s *settingsScreen) View() string {
	theme := s.deps.theme
	rows := []struct {
		label settingsRow
		name  string
	}{
		{rowRepoDir, "Repo directory"},
		{rowAutoUpdate, "Auto update"},
		{rowConfirmRemove, "Confirm remove"},
		{rowTheme, "Theme"},
	}

	out := theme.Title.Render("Settings") + "\n\n"
	for i, r := range rows {
		value := s.renderValue(r.label)
		line := fmt.Sprintf("%-16s %s", r.name, value)
		marker := " "
		if i == s.cursor {
			marker = theme.Cursor()
			line = theme.Selected.Render(strings.TrimSpace(line))
		}
		out += marker + " " + line + "\n"
	}

	out += "\n" + theme.Muted.Render("Changes save automatically when you toggle a value.")

	if s.status != "" {
		out += "\n\n" + theme.Success.Render(s.status)
	}
	if s.err != nil {
		out += "\n\n" + theme.Danger.Render("Error: "+s.err.Error())
	}
	return strings.TrimRight(out, "\n")
}

// renderValue formats one setting's current value for display.
func (s *settingsScreen) renderValue(row settingsRow) string {
	theme := s.deps.theme
	switch row {
	case rowRepoDir:
		if s.editing {
			return s.input.View()
		}
		v := s.local.RepoDirectory
		if v == "" {
			return theme.Muted.Render("(default)")
		}
		return v
	case rowAutoUpdate:
		return renderBool(s.local.AutoUpdate, theme)
	case rowConfirmRemove:
		return renderBool(s.local.ConfirmRemove, theme)
	case rowTheme:
		return s.local.Theme
	}
	return ""
}

// renderBool renders on/off with success/danger styling.
func renderBool(on bool, theme *Theme) string {
	if on {
		return theme.Success.Render("on")
	}
	return theme.Danger.Render("off")
}

// settingsSavedMsg is emitted when a save completes.
type settingsSavedMsg struct {
	saved config.Settings
	err   error
}
