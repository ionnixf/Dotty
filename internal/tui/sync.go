package tui

import (
	"fmt"
	"strings"

	"github.com/ion/dotty/internal/syncer"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
)

// syncScreen runs a sync check, lists problems, and offers per-problem repair.
type syncScreen struct {
	deps     *deps
	sp       spinner.Model
	problems []syncer.Problem
	cursor   int

	state        syncState
	status       string
	err          error
	lastRepaired string
}

// syncState drives what the screen is currently doing.
type syncState int

const (
	syncIdle syncState = iota
	syncChecking
	syncRepairing
)

func newSyncScreen(d *deps) *syncScreen {
	sp := spinner.New(spinner.WithSpinner(spinner.Dot))
	return &syncScreen{deps: d, sp: sp}
}

func (s *syncScreen) title() string { return "Sync" }
func (s *syncScreen) help() string {
	if s.state != syncIdle {
		return "checking … · esc back"
	}
	if len(s.problems) == 0 {
		return "r re-check · esc back"
	}
	return "↑↓ navigate · r repair selected · a repair all · esc back"
}

func (s *syncScreen) enter() {
	// Start an automatic check whenever the screen is shown.
	s.startCheck()
}

func (s *syncScreen) setSize(w, h int) {}

// Init kicks off the check scheduled by enter() and starts the spinner.
// Without this the Sync screen would set its state to syncChecking and hang
// on "Checking installed packages ..." forever, because enter() cannot return
// a command (the screen interface gives it no return value).
func (s *syncScreen) Init() tea.Cmd {
	if s.state == syncIdle {
		return nil
	}
	return tea.Batch(s.sp.Tick, s.runCheck())
}

func (s *syncScreen) startCheck() {
	s.state = syncChecking
	s.status = "Checking installed packages ..."
	s.err = nil
	s.problems = nil
}

func (s *syncScreen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		// ESC/q always leaves the screen, even mid-check/repair. Any late
		// syncCheckedMsg / syncRepairedMsg is ignored by the then-active
		// screen, so leaving early is safe.
		if s.state == syncChecking || s.state == syncRepairing {
			switch keyID(msg) {
			case "esc", "q":
				s.state = syncIdle
				s.status = ""
				return s, back()
			}
			return s, nil
		}
		switch keyID(msg) {
		case "esc", "q":
			s.err = nil
			return s, back()
		case "r":
			if len(s.problems) == 0 {
				// Nothing to repair: treat as re-check.
				s.startCheck()
				return s, tea.Batch(s.sp.Tick, s.runCheck())
			}
			s.lastRepaired = ""
			s.state = syncRepairing
			s.status = "Repairing " + s.problems[s.cursor].Record.Name + " ..."
			problem := s.problems[s.cursor]
			return s, tea.Batch(s.sp.Tick, s.runRepair(problem))
		case "a":
			if len(s.problems) == 0 {
				s.startCheck()
				return s, tea.Batch(s.sp.Tick, s.runCheck())
			}
			s.lastRepaired = ""
			s.state = syncRepairing
			s.status = "Repairing all problems ..."
			toRepair := append([]syncer.Problem(nil), s.problems...)
			return s, tea.Batch(s.sp.Tick, s.runRepairAll(toRepair))
		case "up", "k":
			s.moveCursor(-1)
		case "down", "j":
			s.moveCursor(1)
		}

	case spinner.TickMsg:
		var cmd tea.Cmd
		s.sp, cmd = s.sp.Update(msg)
		if s.state != syncIdle {
			return s, cmd
		}
		return s, nil

	case syncCheckedMsg:
		s.state = syncIdle
		s.problems = msg.problems
		if msg.err != nil {
			s.err = msg.err
			s.status = ""
		} else if len(s.problems) == 0 {
			s.status = "Everything is in sync."
		} else {
			s.status = fmt.Sprintf("Found %d problem(s).", len(s.problems))
		}
		if s.cursor >= len(s.problems) {
			s.cursor = max(0, len(s.problems)-1)
		}
		return s, nil

	case syncRepairedMsg:
		s.state = syncIdle
		s.lastRepaired = msg.repaired
		if msg.err != nil {
			s.err = msg.err
			s.status = ""
		} else if msg.repaired != "" {
			s.status = "Repaired " + msg.repaired
		} else if msg.count > 0 {
			s.status = fmt.Sprintf("Repaired %d package(s).", msg.count)
		}
		// Re-check to refresh the problem list.
		s.startCheck()
		return s, tea.Batch(s.sp.Tick, s.runCheck())
	}

	return s, nil
}

func (s *syncScreen) moveCursor(delta int) {
	if len(s.problems) == 0 {
		return
	}
	s.cursor = (s.cursor + delta + len(s.problems)) % len(s.problems)
}

func (s *syncScreen) runCheck() tea.Cmd {
	syn := s.deps.syncer
	return func() tea.Msg {
		problems, err := syn.Check()
		return syncCheckedMsg{problems: problems, err: err}
	}
}

func (s *syncScreen) runRepair(p syncer.Problem) tea.Cmd {
	syn := s.deps.syncer
	rec := p.Record
	return func() tea.Msg {
		err := syn.Repair(rec)
		return syncRepairedMsg{repaired: rec.Name, err: err}
	}
}

func (s *syncScreen) runRepairAll(ps []syncer.Problem) tea.Cmd {
	syn := s.deps.syncer
	return func() tea.Msg {
		count := 0
		for _, p := range ps {
			if err := syn.Repair(p.Record); err == nil {
				count++
			}
		}
		return syncRepairedMsg{count: count}
	}
}

func (s *syncScreen) View() string {
	theme := s.deps.theme

	var body string
	if s.state != syncIdle {
		body = theme.Title.Render("Sync") + "\n\n" +
			theme.Accent.Render(s.sp.View()) + " " + theme.Muted.Render(s.status)
		if s.err != nil {
			body += "\n\n" + theme.Danger.Render("Error: "+s.err.Error())
		}
		return body
	}

	body = theme.Title.Render("Sync") + "\n\n"

	if len(s.problems) == 0 {
		msg := s.status
		if msg == "" {
			msg = "Everything is in sync."
		}
		body += theme.Success.Render(msg)
		if s.err != nil {
			body += "\n\n" + theme.Danger.Render("Error: "+s.err.Error())
		}
		body += "\n\n" + theme.Muted.Render("Press r to re-check.")
		return body
	}

	body += theme.Warning.Render(s.status) + "\n\n"
	for i, p := range s.problems {
		marker := " "
		line := fmt.Sprintf("%-16s %s  %s",
			p.Record.Name,
			theme.Danger.Render(syncer.KindLabel(p.Kind)),
			p.Detail,
		)
		if i == s.cursor {
			marker = theme.Cursor()
			line = theme.Selected.Render(strings.TrimSpace(line))
		}
		body += marker + " " + line + "\n"
	}
	body += "\n" + theme.Muted.Render("r repair selected · a repair all")
	if s.err != nil {
		body += "\n\n" + theme.Danger.Render("Error: "+s.err.Error())
	}
	return body
}

// syncCheckedMsg is emitted when a sync check finishes.
type syncCheckedMsg struct {
	problems []syncer.Problem
	err      error
}

// syncRepairedMsg is emitted when a repair finishes. repaired holds the
// package name for single repairs; count is set for repair-all.
type syncRepairedMsg struct {
	repaired string
	count    int
	err      error
}
