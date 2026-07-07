package tui

import (
	"errors"
	"fmt"

	"github.com/ion/dotty/internal/catalog"
	"github.com/ion/dotty/internal/config"
	"github.com/ion/dotty/internal/installer"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
)

// installItem is one catalog row in the install list.
type installItem struct {
	pkg       catalog.Package
	installed bool // whether it is already tracked by Dotty
}

func (i installItem) Render() string {
	mark := "  "
	if i.installed {
		mark = "✓ "
	}
	name := i.pkg.Name
	if i.pkg.Description != "" {
		// Keep the row compact: name, then the description dimmed.
		return fmt.Sprintf("%s%-14s %s", mark, name, i.pkg.Description)
	}
	return fmt.Sprintf("%s%s", mark, name)
}

// altItem is one configuration choice in the alternatives list.
type altItem struct {
	alt catalog.ConfigAlternative
}

func (a altItem) Render() string {
	if a.alt.Description != "" {
		return fmt.Sprintf("%-18s %s", a.alt.Name, a.alt.Description)
	}
	return a.alt.Name
}

// installScreen lists installable packages and runs installs.
type installScreen struct {
	deps                *deps
	list                *list
	sp                  spinner.Model
	spinning            bool
	status              string // transient message shown above the footer
	err                 error
	pkgs                []catalog.Package
	showingAlternatives bool
	selectedPkg         catalog.Package
	alternativesList     *list
}

func newInstallScreen(d *deps) *installScreen {
	sp := spinner.New(spinner.WithSpinner(spinner.Dot))
	l := newList("Install", d.theme)
	altL := newList("Select Configuration", d.theme)
	return &installScreen{deps: d, list: l, sp: sp, alternativesList: altL}
}

func (s *installScreen) title() string { return "Install" }
func (s *installScreen) help() string {
	if s.showingAlternatives {
		return "↑↓ navigate · enter select · esc back"
	}
	return "↑↓ navigate · enter install · esc back"
}

// enter reloads the catalog and the installed set so the screen reflects the
// current state each time it becomes active.
func (s *installScreen) enter() {
	s.showingAlternatives = false
	s.err = s.load()
}

func (s *installScreen) setSize(w, h int) {
	s.list.setWidth(w)
	s.list.setHeight(h - 6)
	s.alternativesList.setWidth(w)
	s.alternativesList.setHeight(h - 6)
}

// load fetches catalog + installed records and rebuilds the list items.
func (s *installScreen) load() error {
	pkgs, err := s.deps.reloadCatalog()
	if err != nil {
		s.pkgs = nil
		s.list.setItems(nil)
		return err
	}
	installed, err := s.deps.installedRecords()
	if err != nil {
		s.pkgs = nil
		s.list.setItems(nil)
		return err
	}
	installedSet := make(map[string]bool, len(installed))
	for _, r := range installed {
		installedSet[r.Name] = true
	}
	s.pkgs = pkgs
	items := make([]listItem, 0, len(pkgs))
	for _, p := range pkgs {
		items = append(items, installItem{pkg: p, installed: installedSet[p.Name]})
	}
	s.list.setItems(items)
	return nil
}

func (s *installScreen) Init() tea.Cmd { return nil }

func (s *installScreen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if s.showingAlternatives {
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch keyID(msg) {
			case "esc", "q":
				s.showingAlternatives = false
				return s, nil
			case "enter":
				if s.alternativesList.selected() == nil {
					return s, nil
				}
				alt := s.alternativesList.selected().(altItem).alt
				s.showingAlternatives = false
				s.status = "Installing " + s.selectedPkg.Name + " (" + alt.Name + ") ..."
				s.err = nil
				s.spinning = true
				req := installer.Request{
					Name:   s.selectedPkg.Name,
					Repo:   alt.Repo,
					Source: alt.Source,
					Target: s.selectedPkg.Target,
				}
				return s, tea.Batch(s.sp.Tick, s.runInstall(req))
			}
			if s.alternativesList.handleKey(msg) {
				return s, nil
			}
		}
		return s, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch keyID(msg) {
		case "esc", "q":
			if s.spinning {
				return s, nil // ignore navigation while an install is running
			}
			s.status = ""
			s.err = nil
			return s, back()
		case "enter":
			if s.spinning || s.list.selected() == nil {
				return s, nil
			}
			it := s.list.selected().(installItem)
			if len(it.pkg.Alternatives) > 0 {
				s.showingAlternatives = true
				s.selectedPkg = it.pkg
				s.alternativesList.title = "Select Configuration for " + it.pkg.Name
				items := make([]listItem, len(it.pkg.Alternatives))
				for idx, alt := range it.pkg.Alternatives {
					items[idx] = altItem{alt: alt}
				}
				s.alternativesList.setItems(items)
				s.alternativesList.cursor = 0
				return s, nil
			}

			s.status = "Installing " + it.pkg.Name + " ..."
			s.err = nil
			s.spinning = true
			req := installer.Request{
				Name:   it.pkg.Name,
				Repo:   it.pkg.Repo,
				Source: it.pkg.Source,
				Target: it.pkg.Target,
			}
			return s, tea.Batch(s.sp.Tick, s.runInstall(req))
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

	case installDoneMsg:
		s.spinning = false
		if msg.err != nil {
			s.err = msg.err
			s.status = ""
		} else {
			s.status = "Installed " + msg.res.Name + " → " + shortTarget(s.deps, msg.res.Target)
		}
		// Refresh the list so the freshly installed package shows a check.
		if err := s.load(); err != nil {
			s.err = err
		}
		return s, nil
	}

	return s, nil
}

// friendlyInstallError turns the installer's wrapped errors into a short,
// actionable message. The raw error is preserved for unexpected cases.
func friendlyInstallError(err error) string {
	if errors.Is(err, installer.ErrTargetExists) {
		return "target already exists (back it up or remove it, then retry)"
	}
	return err.Error()
}

// runInstall performs the actual install on a background goroutine. It must
// not touch the model; it returns an installDoneMsg.
func (s *installScreen) runInstall(req installer.Request) tea.Cmd {
	inst := s.deps.installer
	return func() tea.Msg {
		res, err := inst.Install(req)
		if err != nil {
			return installDoneMsg{err: err}
		}
		return installDoneMsg{res: res}
	}
}

func (s *installScreen) View() string {
	var body string
	if s.showingAlternatives {
		body = s.alternativesList.render()
	} else {
		body = s.list.render()
	}
	if s.spinning {
		body += "\n\n" + s.deps.theme.Accent.Render(s.sp.View()) + " " + s.deps.theme.Muted.Render(s.status)
	} else if s.status != "" {
		body += "\n\n" + s.deps.theme.Success.Render(s.status)
	}
	if s.err != nil {
		body += "\n\n" + s.deps.theme.Danger.Render("Error: "+friendlyInstallError(s.err))
	}
	return body
}

// installDoneMsg is emitted when a background install finishes.
type installDoneMsg struct {
	res installer.Result
	err error
}

// shortTarget renders an absolute target path in ~ form for display.
func shortTarget(d *deps, abs string) string {
	return config.Shorten(abs, d.home)
}
