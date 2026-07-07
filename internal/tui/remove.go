package tui

import (
	"errors"
	"fmt"

	"github.com/ion/dotty/internal/remover"
	"github.com/ion/dotty/internal/storage"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
)

// removeItem is one installed package in the remove list.
type removeItem struct{ rec storage.Record }

func (i removeItem) Render() string {
	return fmt.Sprintf("%-14s %s", i.rec.Name, i.rec.Target)
}

// removeScreen lists installed packages and removes the selection after a
// confirmation. The confirmation is a simple two-option inline overlay driven
// by pendingRemove and confirmYes.
type removeScreen struct {
	deps *deps
	list *list
	sp   spinner.Model

	pendingRemove *string // name awaiting confirmation, nil when not confirming
	confirmYes    bool
	lastAttempt   string // name of the most recent remove attempt

	spinning bool
	status   string
	err      error
}

func newRemoveScreen(d *deps) *removeScreen {
	sp := spinner.New(spinner.WithSpinner(spinner.Dot))
	l := newList("Remove", d.theme)
	return &removeScreen{deps: d, list: l, sp: sp}
}

func (s *removeScreen) title() string { return "Remove" }
func (s *removeScreen) help() string {
	if s.pendingRemove != nil {
		return "← → choose · enter confirm · esc cancel"
	}
	return "↑↓ navigate · enter remove · esc back"
}

func (s *removeScreen) enter() { s.err = s.load() }

func (s *removeScreen) setSize(w, h int) {
	s.list.setWidth(w)
	s.list.setHeight(h - 6)
}

func (s *removeScreen) load() error {
	records, err := s.deps.installedRecords()
	if err != nil {
		s.list.setItems(nil)
		return err
	}
	items := make([]listItem, 0, len(records))
	for _, r := range records {
		items = append(items, removeItem{rec: r})
	}
	s.list.setItems(items)
	return nil
}

func (s *removeScreen) Init() tea.Cmd { return nil }

func (s *removeScreen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Confirmation overlay takes priority over list navigation.
	if s.pendingRemove != nil {
		return s.handleConfirm(msg)
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch keyID(msg) {
		case "esc", "q":
			s.reset()
			return s, back()
		case "enter":
			if s.spinning || s.list.selected() == nil {
				return s, nil
			}
			it := s.list.selected().(removeItem)
			// ConfirmRemove setting gates whether we ask first.
			if s.deps.settings.ConfirmRemove {
				name := it.rec.Name
				s.pendingRemove = &name
				s.confirmYes = false
				return s, nil
			}
			return s, s.startRemove(it.rec.Name)
		}
		if s.list.handleKey(msg) {
			return s, nil
		}

	case spinner.TickMsg:
		var cmd tea.Cmd
		s.sp, cmd = s.sp.Update(msg)
		if s.spinning {
			return s, cmd
		}
		return s, nil

	case removeDoneMsg:
		s.spinning = false
		if msg.err != nil {
			s.err = msg.err
			s.status = ""
		} else {
			s.status = "Removed " + msg.name
		}
		if err := s.load(); err != nil {
			s.err = err
		}
		return s, nil
	}

	return s, nil
}

// handleConfirm routes keys while the yes/no overlay is open.
func (s *removeScreen) handleConfirm(msg tea.Msg) (tea.Model, tea.Cmd) {
	kmsg, ok := msg.(tea.KeyMsg)
	if !ok {
		// Allow spinner ticks through.
		if tick, ok := msg.(spinner.TickMsg); ok {
			var cmd tea.Cmd
			s.sp, cmd = s.sp.Update(tick)
			return s, cmd
		}
		if done, ok := msg.(removeDoneMsg); ok {
			s.spinning = false
			if done.err != nil {
				s.err = done.err
				s.status = ""
			} else {
				s.status = "Removed " + done.name
			}
			s.pendingRemove = nil
			if err := s.load(); err != nil {
				s.err = err
			}
			return s, nil
		}
		return s, nil
	}

	switch keyID(kmsg) {
	case "left", "h":
		s.confirmYes = true
	case "right", "l":
		s.confirmYes = false
	case "y":
		name := *s.pendingRemove
		s.pendingRemove = nil
		return s, s.startRemove(name)
	case "n", "esc", "q":
		s.pendingRemove = nil
		return s, nil
	case "enter":
		name := *s.pendingRemove
		confirm := s.confirmYes
		s.pendingRemove = nil
		if !confirm {
			return s, nil
		}
		return s, s.startRemove(name)
	}
	return s, nil
}

func (s *removeScreen) reset() {
	s.status = ""
	s.err = nil
	s.pendingRemove = nil
}

func (s *removeScreen) startRemove(name string) tea.Cmd {
	s.reset()
	s.lastAttempt = name
	s.status = "Removing " + name + " ..."
	s.err = nil
	s.spinning = true
	rem := s.deps.remover
	return tea.Batch(s.sp.Tick, func() tea.Msg {
		err := rem.Remove(name)
		return removeDoneMsg{name: name, err: err}
	})
}

func (s *removeScreen) View() string {
	body := s.list.render()

	if s.pendingRemove != nil {
		yes := "  Yes"
		no := "No  "
		if s.confirmYes {
			yes = s.deps.theme.Success.Render("› Yes")
		} else {
			no = s.deps.theme.Danger.Render("No ‹")
		}
		prompt := s.deps.theme.Warning.Render("Remove "+*s.pendingRemove+"?") +
			"\n\n" + yes + "   " + no
		body += "\n\n" + prompt
	}

	if s.spinning {
		body += "\n\n" + s.deps.theme.Accent.Render(s.sp.View()) + " " + s.deps.theme.Muted.Render(s.status)
	} else if s.status != "" {
		body += "\n\n" + s.deps.theme.Success.Render(s.status)
	}
	if s.err != nil {
		// Expected failures get a friendlier message; everything else is
		// shown verbatim so real problems stay debuggable.
		msg := s.err.Error()
		switch {
		case errors.Is(s.err, remover.ErrNotFound):
			msg = s.lastAttempt + " is not installed"
		case errors.Is(s.err, remover.ErrNotASymlink):
			msg = s.lastAttempt + " target is not a Dotty-managed symlink; not removed"
		}
		body += "\n\n" + s.deps.theme.Danger.Render("Error: "+msg)
	}
	return body
}

// removeDoneMsg is emitted when a background remove finishes.
type removeDoneMsg struct {
	name string
	err  error
}
