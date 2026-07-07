package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// listItem is a single selectable row. Render returns the row's body text
// (without the cursor); the list applies selection styling.
type listItem interface {
	Render() string
}

// list is a minimal selectable list widget with scrolling/viewport windowing.
// It owns a cursor, a viewport window, and handles key navigation.
//
// theme is a pointer so a live theme change is reflected by every list on its
// next render without rebuilding the widgets.
type list struct {
	title      string
	items      []listItem
	cursor     int
	startIndex int
	maxHeight  int
	theme      *Theme
	width      int
	help       string // shown in the footer when non-empty
}

// newList builds a list with a title and theme. Items are set via setItems.
func newList(title string, theme *Theme) *list {
	return &list{title: title, theme: theme, cursor: 0}
}

// setItems replaces the list contents, clamps the cursor, and resets viewport.
func (l *list) setItems(items []listItem) {
	l.items = items
	if l.cursor >= len(l.items) {
		l.cursor = max(0, len(l.items)-1)
	}
	l.adjustViewport()
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

// setHeight sets the maximum number of items visible at once.
func (l *list) setHeight(h int) {
	l.maxHeight = h
	l.adjustViewport()
}

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
	l.adjustViewport()
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
	l.adjustViewport()
}

// top moves the cursor to the first item.
func (l *list) top() {
	l.cursor = 0
	l.adjustViewport()
}

// bottom moves the cursor to the last item.
func (l *list) bottom() {
	if len(l.items) > 0 {
		l.cursor = len(l.items) - 1
	}
	l.adjustViewport()
}

// adjustViewport keeps the cursor within the visible window.
func (l *list) adjustViewport() {
	if l.maxHeight <= 0 || len(l.items) <= l.maxHeight {
		l.startIndex = 0
		return
	}
	if l.cursor >= l.startIndex+l.maxHeight {
		l.startIndex = l.cursor - l.maxHeight + 1
	}
	if l.cursor < l.startIndex {
		l.startIndex = l.cursor
	}
}

// render produces the body of the list, rendering only the visible items
// according to the current viewport windowing.
func (l *list) render() string {
	if len(l.items) == 0 {
		empty := l.theme.Muted.Render("(no entries)")
		return l.theme.Title.Render(l.title) + "\n\n" + empty
	}

	rows := make([]string, 0, len(l.items)+3)
	rows = append(rows, l.theme.Title.Render(l.title))
	rows = append(rows, "")

	l.adjustViewport()

	endIndex := len(l.items)
	if l.maxHeight > 0 && l.startIndex+l.maxHeight < endIndex {
		endIndex = l.startIndex + l.maxHeight
	}

	for i := l.startIndex; i < endIndex; i++ {
		it := l.items[i]
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

	// Add a subtle scroll indicator at the bottom if list is larger than viewport
	if l.maxHeight > 0 && len(l.items) > l.maxHeight {
		scrollInfo := fmt.Sprintf(" %d-%d of %d ", l.startIndex+1, endIndex, len(l.items))
		rows = append(rows, "", l.theme.Muted.Render(scrollInfo))
	}

	return lipgloss.JoinVertical(lipgloss.Left, rows...)
}

// handleKey routes up/down/j/k navigation. Returns true if the key was
// consumed.
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
