package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// listItem is a single selectable row. Render returns the row's body text
// (without the cursor); the list applies selection styling.
type listItem interface {
	Render() string
}

// list is a minimal selectable list widget. It owns a cursor, a fixed set of
// items, and rendering with the theme's cursor glyph. We deliberately do not
// use bubbles/list: its filter/pagination/keybindings fight the navigational
// style this app needs, and a 60-line widget is simpler than fighting it.
type list struct {
	title  string
	items  []listItem
	cursor int
	theme  Theme
	width  int
	help   string // shown in the footer when non-empty
}

// newList builds a list with a title and theme. Items are set via setItems.
func newList(title string, theme Theme) *list {
	return &list{title: title, theme: theme, cursor: 0}
}

// setItems replaces the list contents and clamps the cursor.
func (l *list) setItems(items []listItem) {
	l.items = items
	if l.cursor >= len(l.items) {
		l.cursor = max(0, len(l.items)-1)
	}
}

// count returns the number of items.
func (l *list) count() int { return len(l.items) }

// selected returns the current item, or nil if the list is empty.
func (l *list) selected() listItem {
	if len(l.items) == 0 {
		return nil
	}
	return l.items[l.cursor]
}

// selectedIndex returns the current cursor position.
func (l *list) selectedIndex() int { return l.cursor }

// setWidth stores the available width for rendering.
func (l *list) setWidth(w int) { l.width = w }

// up moves the cursor toward the top, wrapping at the edges.
func (l *list) up() {
	if len(l.items) == 0 {
		return
	}
	if l.cursor == 0 {
		l.cursor = len(l.items) - 1
	} else {
		l.cursor--
	}
}

// down moves the cursor toward the bottom, wrapping at the edges.
func (l *list) down() {
	if len(l.items) == 0 {
		return
	}
	if l.cursor == len(l.items)-1 {
		l.cursor = 0
	} else {
		l.cursor++
	}
}

// top moves the cursor to the first item.
func (l *list) top() { l.cursor = 0 }

// bottom moves the cursor to the last item.
func (l *list) bottom() {
	if len(l.items) > 0 {
		l.cursor = len(l.items) - 1
	}
}

// render produces the body of the list: title row followed by items with the
// cursor marker. It does not draw a border; the caller wraps it in a panel.
func (l *list) render() string {
	if len(l.items) == 0 {
		empty := l.theme.Muted.Render("(no entries)")
		return l.theme.Title.Render(l.title) + "\n\n" + empty
	}

	rows := make([]string, 0, len(l.items)+1)
	rows = append(rows, l.theme.Title.Render(l.title))
	rows = append(rows, "")

	for i, it := range l.items {
		marker := " "
		body := it.Render()
		if i == l.cursor {
			marker = l.theme.Cursor()
			body = l.theme.Selected.Render(body)
		} else {
			body = l.theme.Status.Render(body)
		}
		rows = append(rows, marker+" "+body)
	}
	return lipgloss.JoinVertical(lipgloss.Left, rows...)
}

// handleKey routes up/down/j/k navigation. Returns true if the key was
// consumed so callers can fall through for other keys.
func (l *list) handleKey(msg tea.KeyMsg) bool {
	switch keyID(msg) {
	case "up", "k":
		l.up()
		return true
	case "down", "j":
		l.down()
		return true
	case "g":
		l.top()
		return true
	case "G":
		l.bottom()
		return true
	}
	return false
}
