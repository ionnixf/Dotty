package tui

import (
	tea "github.com/charmbracelet/bubbletea"
)

// menuItem is one row of the main menu.
type menuItem struct {
	label string
	dest  screenKind
}

func (m menuItem) Render() string { return m.label }

// menuScreen is the top-level navigation screen.
type menuScreen struct {
	deps *deps
	list *list
}

func newMenuScreen(d *deps) *menuScreen {
	l := newList("Dotty", d.theme)
	items := []listItem{
		menuItem{"Install", screenInstall},
		menuItem{"Update", screenUpdate},
		menuItem{"Remove", screenRemove},
		menuItem{"Installed", screenInstalled},
		menuItem{"Sync", screenSync},
		menuItem{"Import Existing", screenImport},
		menuItem{"Settings", screenSettings},
		menuItem{"Exit", 0}, // dest 0 is unused; handled specially
	}
	l.setItems(items)
	return &menuScreen{deps: d, list: l}
}

func (s *menuScreen) title() string { return "Dotty" }
func (s *menuScreen) help() string  { return "↑↓ navigate · enter select · q quit" }
func (s *menuScreen) enter()        {}
func (s *menuScreen) Init() tea.Cmd { return nil }

func (s *menuScreen) setSize(w, h int) {
	s.list.setWidth(w)
	s.list.setHeight(h - 10)
}

func (s *menuScreen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch keyID(msg) {
		case "enter":
			it := s.list.selected()
			if it == nil {
				return s, nil
			}
			mi := it.(menuItem)
			if mi.label == "Exit" {
				return s, quit()
			}
			return s, nav(mi.dest)
		case "q", "esc", "ctrl+c":
			return s, quit()
		}
		if s.list.handleKey(msg) {
			return s, nil
		}
	}
	return s, nil
}

func (s *menuScreen) View() string {
	return s.list.render()
}
