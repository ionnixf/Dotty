package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// installedScreen renders the installed packages as a three-column table with
// live status (whether the symlink currently resolves). It is read-only; the
// cursor only highlights rows.
type installedScreen struct {
	deps   *deps
	rows   []installedRow
	cursor int
	err    error
}

// installedRow holds the display data for one installed package.
type installedRow struct {
	name   string
	status string
	target string
}

func newInstalledScreen(d *deps) *installedScreen {
	return &installedScreen{deps: d}
}

func (s *installedScreen) title() string { return "Installed" }
func (s *installedScreen) help() string  { return "↑↓ navigate · esc back" }
func (s *installedScreen) enter()        { s.err = s.load() }
func (s *installedScreen) Init() tea.Cmd { return nil }

// load reads the installed database and computes each row's live status by
// checking the symlink on disk.
func (s *installedScreen) load() error {
	records, err := s.deps.installedRecords()
	if err != nil {
		s.rows = nil
		return err
	}
	rows := make([]installedRow, 0, len(records))
	for _, r := range records {
		status := liveStatus(s.deps, r)
		rows = append(rows, installedRow{
			name:   r.Name,
			status: status,
			target: r.Target,
		})
	}
	s.rows = rows
	if s.cursor >= len(s.rows) {
		s.cursor = max(0, len(s.rows)-1)
	}
	return nil
}

func (s *installedScreen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch keyID(msg) {
		case "esc", "q":
			s.err = nil
			return s, back()
		case "up", "k":
			if len(s.rows) > 0 {
				if s.cursor == 0 {
					s.cursor = len(s.rows) - 1
				} else {
					s.cursor--
				}
			}
		case "down", "j":
			if len(s.rows) > 0 {
				if s.cursor == len(s.rows)-1 {
					s.cursor = 0
				} else {
					s.cursor++
				}
			}
		}
	}
	return s, nil
}

// View renders the table with fixed-width columns. Column widths are chosen
// to look right in typical terminals; wide names/targets are truncated.
func (s *installedScreen) View() string {
	const (
		colName   = 14
		colStatus = 12
	)

	// Header row.
	head := fmt.Sprintf("%-*s  %-*s  %s",
		colName, s.deps.theme.RowHead.Render("Package"),
		colStatus, s.deps.theme.RowHead.Render("Status"),
		s.deps.theme.RowHead.Render("Target"),
	)
	body := head + "\n"

	if len(s.rows) == 0 {
		body += "\n" + s.deps.theme.Muted.Render("(nothing installed yet)")
		return body
	}

	for i, row := range s.rows {
		name := truncate(row.name, colName)
		status := truncate(row.status, colStatus)
		target := row.target

		var statusStyled string
		switch row.status {
		case "Installed":
			statusStyled = s.deps.theme.Success.Render(status)
		case "Broken":
			statusStyled = s.deps.theme.Danger.Render(status)
		case "Missing":
			statusStyled = s.deps.theme.Warning.Render(status)
		default:
			statusStyled = s.deps.theme.Muted.Render(status)
		}

		line := fmt.Sprintf("%-*s  %-*s  %s", colName, name, colStatus, statusStyled, target)
		if i == s.cursor {
			line = s.deps.theme.Cursor() + " " + s.deps.theme.Selected.Render(strings.TrimSpace(line))
		} else {
			line = "  " + line
		}
		body += line + "\n"
	}

	if s.err != nil {
		body += "\n" + s.deps.theme.Danger.Render("Error: "+s.err.Error())
	}
	return strings.TrimRight(body, "\n")
}

// truncate shortens s to width runes, appending an ellipsis if it was cut.
func truncate(s string, width int) string {
	r := []rune(s)
	if len(r) <= width {
		return s
	}
	return string(r[:width-1]) + "…"
}
