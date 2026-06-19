package tui

import (
	"fmt"

	"github.com/ion/dotty/internal/config"
	"github.com/ion/dotty/internal/scanner"
	"github.com/ion/dotty/internal/storage"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
)

// importedRepoTag marks a record that was added by the Import Existing flow
// rather than cloned from a remote. The Sync screen still treats it like any
// other record; its symlink is already on disk pointing at the real config.
const importedRepoTag = "imported:existing"

// importItem is one discovered config in the import list.
type importItem struct {
	find    scanner.Find
	tracked bool // whether Dotty already tracks this target
}

func (i importItem) Render() string {
	mark := "  "
	if i.tracked {
		mark = "✓ "
	}
	return fmt.Sprintf("%s%-14s %s", mark, i.find.Name, i.find.Target)
}

// importScreen scans ~/.config for existing configs the user can adopt.
type importScreen struct {
	deps   *deps
	list   *list
	sp     spinner.Model
	state  importState
	finds  []scanner.Find
	status string
	err    error
}

type importState int

const (
	importIdle importState = iota
	importScanning
	importing
)

func newImportScreen(d *deps) *importScreen {
	sp := spinner.New(spinner.WithSpinner(spinner.Dot))
	l := newList("Import Existing", d.theme)
	return &importScreen{deps: d, list: l, sp: sp}
}

func (s *importScreen) title() string { return "Import Existing" }
func (s *importScreen) help() string {
	if s.state != importIdle {
		return "scanning … · esc back"
	}
	if s.list.count() == 0 {
		return "r re-scan · esc back"
	}
	return "↑↓ navigate · enter import · r re-scan · esc back"
}

// enter triggers a scan each time the screen becomes active. It only sets up
// state; the command that actually runs the scan is returned by Init, which
// the app dispatches via switchTo. Returning the command from enter() would
// not work because the screen interface's enter() has no return value.
func (s *importScreen) enter() {
	s.startScan()
}

// Init kicks off the scan scheduled by enter() and starts the spinner. This
// is what makes the Import screen actually progress instead of hanging on
// "Scanning ~/.config ..." forever.
func (s *importScreen) Init() tea.Cmd {
	if s.state == importIdle {
		return nil
	}
	return tea.Batch(s.sp.Tick, s.runScan())
}

func (s *importScreen) startScan() {
	s.state = importScanning
	s.status = "Scanning ~/.config ..."
	s.err = nil
	s.finds = nil
	s.list.setItems(nil)
}

// runScan performs the scan off the main loop so the spinner can animate.
func (s *importScreen) runScan() tea.Cmd {
	dir := s.deps.configDir
	return func() tea.Msg {
		finds, err := scanner.Scan(dir)
		return importScannedMsg{finds: finds, err: err}
	}
}

// load rebuilds the list from the last scan plus the tracked set.
func (s *importScreen) load() error {
	installed, err := s.deps.installedRecords()
	if err != nil {
		s.list.setItems(nil)
		return err
	}
	tracked := make(map[string]bool, len(installed))
	for _, r := range installed {
		tracked[r.Target] = true
	}
	items := make([]listItem, 0, len(s.finds))
	for _, f := range s.finds {
		items = append(items, importItem{find: f, tracked: tracked[f.Target]})
	}
	s.list.setItems(items)
	return nil
}

func (s *importScreen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		// ESC/q always leaves the screen, even mid-scan. A late
		// importScannedMsg arriving afterwards is simply ignored by the
		// then-active screen, so there is no harm in leaving early.
		if s.state == importScanning || s.state == importing {
			if keyID(msg) == "esc" || keyID(msg) == "q" {
				s.state = importIdle
				s.status = ""
				return s, back()
			}
			return s, nil
		}
		switch keyID(msg) {
		case "esc", "q":
			s.err = nil
			s.status = ""
			return s, back()
		case "r":
			s.startScan()
			return s, tea.Batch(s.sp.Tick, s.runScan())
		case "enter":
			if s.list.selected() == nil {
				return s, nil
			}
			it := s.list.selected().(importItem)
			s.state = importing
			s.status = "Importing " + it.find.Name + " ..."
			s.err = nil
			return s, tea.Batch(s.sp.Tick, s.runImport(it.find))
		}
		if s.list.handleKey(msg) {
			return s, nil
		}

	case spinner.TickMsg:
		var cmd tea.Cmd
		s.sp, cmd = s.sp.Update(msg)
		if s.state != importIdle {
			return s, cmd
		}
		return s, nil

	case importScannedMsg:
		s.state = importIdle
		if msg.err != nil {
			s.err = msg.err
			s.status = ""
			s.finds = nil
			s.list.setItems(nil)
			return s, nil
		}
		s.finds = msg.finds
		if len(s.finds) == 0 {
			s.status = "No existing configs found."
		} else {
			s.status = ""
		}
		if err := s.load(); err != nil {
			s.err = err
		}
		return s, nil

	case importDoneMsg:
		s.state = importIdle
		if msg.err != nil {
			s.err = msg.err
			s.status = ""
		} else {
			s.status = "Imported " + msg.name + " — now tracked by Dotty."
		}
		if err := s.load(); err != nil {
			s.err = err
		}
		return s, nil
	}

	return s, nil
}

// runImport records an existing config so Dotty begins tracking it. The
// config directory already exists on disk; we add a Record describing it.
func (s *importScreen) runImport(f scanner.Find) tea.Cmd {
	store := s.deps.store
	home := s.deps.home
	return func() tea.Msg {
		rec := storage.Record{
			Name:   f.Name,
			Repo:   importedRepoTag,
			Source: "",
			Target: config.Shorten(f.Target, home),
		}
		if err := store.Add(rec); err != nil {
			return importDoneMsg{name: f.Name, err: err}
		}
		return importDoneMsg{name: f.Name}
	}
}

func (s *importScreen) View() string {
	body := s.list.render()
	if s.state != importIdle {
		body += "\n\n" + s.deps.theme.Accent.Render(s.sp.View()) + " " + s.deps.theme.Muted.Render(s.status)
	} else if s.status != "" {
		body += "\n\n" + s.deps.theme.Success.Render(s.status)
	}
	if s.err != nil {
		body += "\n\n" + s.deps.theme.Danger.Render("Error: "+s.err.Error())
	}
	return body
}

// importScannedMsg carries the result of a scan.
type importScannedMsg struct {
	finds []scanner.Find
	err   error
}

// importDoneMsg carries the result of adopting one config.
type importDoneMsg struct {
	name string
	err  error
}
