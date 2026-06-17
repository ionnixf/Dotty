// Package tui implements Dotty's terminal interface with Bubble Tea and
// Lip Gloss. It is strictly a presentation layer: every mutating action is
// delegated to a domain package via a deps struct, so the logic stays
// testable without a terminal.
package tui

import (
	tea "github.com/charmbracelet/bubbletea"
)

// keyID returns a stable, lowercased string identifying the user-meaningful
// key for a KeyMsg. Bubble Tea's KeyMsg.String() is already designed for
// direct comparison ("up", "down", "enter", "esc", "q", "ctrl+c", ...);
// we only normalise case so callers can write keyID(msg) == "Q".
func keyID(msg tea.KeyMsg) string {
	return msg.String()
}
