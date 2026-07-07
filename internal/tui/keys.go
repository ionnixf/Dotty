// Package tui implements Dotty's terminal interface with Bubble Tea and
// Lip Gloss. It is strictly a presentation layer: every mutating action is
// delegated to a domain package via a deps struct, so the logic stays
// testable without a terminal.
package tui

import (
	tea "github.com/charmbracelet/bubbletea"
)

// keyID returns the stable string representation identifying the user-meaningful
// key for a KeyMsg. Bubble Tea's KeyMsg.String() is designed for direct
// comparison ("up", "down", "enter", "esc", "q", "g", "G", "ctrl+c", ...).
// Casing is preserved so that case-sensitive keybindings (like 'g' for top
// and 'G' for bottom) function correctly.
func keyID(msg tea.KeyMsg) string {
	return msg.String()
}
