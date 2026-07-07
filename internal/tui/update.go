package tui

import (
	"fmt"
	"strings"

	"github.com/ion/dotty/internal/storage"
	"github.com/ion/dotty/internal/updater"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
)

// updateItem is one row in the update list. The "Update All" entry is a
// sentinel with updateAllSentinel set.
type updateItem struct {
	rec storage.Record
	all bool
}

func (i updateItem) Render() string {
	if i.all {
		return "Update All"
	}
	return fmt.Sprintf("%-14s %s", i.rec.Name, i.rec.Target)
}

// updateScreen lists installed packages and runs git pull on the selection.
type updateScreen struct {
	deps     *deps
	list     *list
	sp       spinner.Model
	spinning bool
	status   string
	err      error
	results  string // captured git output to display
}

func newUpdateScreen(d *deps) *updateScreen {
	sp := spinner.New(spinner.WithSpinner(spinner.Dot))
	l := newList("Update", d.theme)
	return &updateScreen{deps: d, list: l, sp: sp}
}

func (s *updateScreen) title() string { return "Update" }
func (s *updateScreen) help() string {
	return "↑↓ navigate · enter update · a update all · esc back"
}

func (s *updateScreen) enter() {
	s.err = s.load()
}

func (s *updateScreen) setSize(w, h int) {
	s.list.setWidth(w)
	s.list.setHeight(h - 10)
}

// load rebuilds the list from the installed database, prepending "Update All".
func (s *updateScreen) load() error {
	records, err := s.deps.installedRecords()
	if err != nil {
		s.list.setItems(nil)
		return err
	}
	items := make([]listItem, 0, len(records)+1)
	if len(records) > 0 {
		items = append(items, updateItem{all: true})
	}
	for _, r := range records {
		items = append(items, updateItem{rec: r})
	}
	s.list.setItems(items)
	return nil
}

func (s *updateScreen) Init() tea.Cmd { return nil }

func (s *updateScreen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch keyID(msg) {
		case "esc", "q":
			if s.spinning {
				return s, nil
			}
			s.reset()
			return s, back()
		case "a":
			if s.spinning {
				return s, nil
			}
			s.startAll()
			return s, tea.Batch(s.sp.Tick, s.runUpdateAll())
		case "enter":
			if s.spinning || s.list.selected() == nil {
				return s, nil
			}
			it := s.list.selected().(updateItem)
			if it.all {
				s.startAll()
				return s, tea.Batch(s.sp.Tick, s.runUpdateAll())
			}
			s.startOne(it.rec.Name)
			return s, tea.Batch(s.sp.Tick, s.runUpdateOne(it.rec.Name))
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

	case updateDoneMsg:
		s.spinning = false
		s.results = renderOutcomes(msg.outcomes)
		if msg.err != nil {
			s.err = msg.err
			s.status = ""
		} else {
			s.status = fmt.Sprintf("Updated %d package(s).", len(msg.outcomes))
		}
		return s, nil
	}

	return s, nil
}

func (s *updateScreen) reset() {
	s.status = ""
	s.err = nil
	s.results = ""
}

func (s *updateScreen) startOne(name string) {
	s.reset()
	s.status = "Updating " + name + " ..."
	s.spinning = true
}

func (s *updateScreen) startAll() {
	s.reset()
	s.status = "Updating all packages ..."
	s.spinning = true
}

func (s *updateScreen) runUpdateOne(name string) tea.Cmd {
	upd := s.deps.updater
	return func() tea.Msg {
		o := upd.Update(name)
		return updateDoneMsg{outcomes: []updater.Outcome{o}}
	}
}

func (s *updateScreen) runUpdateAll() tea.Cmd {
	upd := s.deps.updater
	return func() tea.Msg {
		outcomes, err := upd.UpdateAll()
		if err != nil {
			return updateDoneMsg{err: err}
		}
		return updateDoneMsg{outcomes: outcomes}
	}
}

func (s *updateScreen) View() string {
	body := s.list.render()
	if s.spinning {
		body += "\n\n" + s.deps.theme.Accent.Render(s.sp.View()) + " " + s.deps.theme.Muted.Render(s.status)
	} else if s.status != "" {
		body += "\n\n" + s.deps.theme.Success.Render(s.status)
	}
	if s.results != "" {
		body += "\n\n" + s.results
	}
	if s.err != nil {
		body += "\n\n" + s.deps.theme.Danger.Render("Error: "+s.err.Error())
	}
	return body
}

// updateDoneMsg is emitted when a background update finishes.
type updateDoneMsg struct {
	outcomes []updater.Outcome
	err      error
}

// renderOutcomes formats one or more update results for display.
func renderOutcomes(os []updater.Outcome) string {
	var b strings.Builder
	for _, o := range os {
		if o.Err != nil {
			fmt.Fprintf(&b, "%s: %s\n", o.Name, o.Err.Error())
		} else {
			out := strings.TrimSpace(o.Output)
			if out == "" || strings.Contains(strings.ToLower(out), "already up to date") {
				fmt.Fprintf(&b, "%s: up to date\n", o.Name)
			} else {
				fmt.Fprintf(&b, "%s:\n", o.Name)
				for _, line := range strings.Split(out, "\n") {
					fmt.Fprintf(&b, "  %s\n", line)
				}
			}
		}
	}
	return strings.TrimRight(b.String(), "\n")
}
