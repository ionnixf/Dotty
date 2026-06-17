package tui

import "github.com/charmbracelet/lipgloss"

// palette is a set of ANSI colours for one theme.
type palette struct {
	Primary    lipgloss.Color // brand / titles
	Accent     lipgloss.Color // selection accents
	Muted      lipgloss.Color // secondary text
	Success    lipgloss.Color // installed / ok
	Warning    lipgloss.Color // problems / confirmations
	Danger     lipgloss.Color // remove / errors
	Border     lipgloss.Color // panel borders
	Background lipgloss.Color // header background
}

var palettes = map[string]palette{
	"dark": {
		Primary:    lipgloss.Color("213"),
		Accent:     lipgloss.Color("117"),
		Muted:      lipgloss.Color("245"),
		Success:    lipgloss.Color("114"),
		Warning:    lipgloss.Color("221"),
		Danger:     lipgloss.Color("203"),
		Border:     lipgloss.Color("240"),
		Background: lipgloss.Color("236"),
	},
	"light": {
		Primary:    lipgloss.Color("91"),
		Accent:     lipgloss.Color("25"),
		Muted:      lipgloss.Color("242"),
		Success:    lipgloss.Color("22"),
		Warning:    lipgloss.Color("130"),
		Danger:     lipgloss.Color("124"),
		Border:     lipgloss.Color("250"),
		Background: lipgloss.Color("253"),
	},
}

// CursorGlyph is the rune drawn before the selected list row.
const CursorGlyph = "›"

// Theme groups all styles Dotty uses, derived from a palette. Styles are
// immutable lipgloss styles; views compose them via Render and the cursor
// glyph rendered with cursorStyle.
type Theme struct {
	Name        string
	Header      lipgloss.Style
	Title       lipgloss.Style
	Subtitle    lipgloss.Style
	cursorStyle lipgloss.Style
	Panel       lipgloss.Style
	RowHead     lipgloss.Style
	Selected    lipgloss.Style
	Muted       lipgloss.Style
	Accent      lipgloss.Style
	Success     lipgloss.Style
	Warning     lipgloss.Style
	Danger      lipgloss.Style
	Footer      lipgloss.Style
	Status      lipgloss.Style
}

// Cursor renders the selection marker with the theme's primary colour.
func (t Theme) Cursor() string { return t.cursorStyle.Render(CursorGlyph) }

// NewTheme builds a Theme from name ("dark" or "light"). Unknown names fall
// back to "dark", matching config.NormaliseTheme.
func NewTheme(name string) Theme {
	pal, ok := palettes[name]
	if !ok {
		pal = palettes["dark"]
		name = "dark"
	}

	panel := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(pal.Border).
		Padding(0, 1)

	return Theme{
		Name: name,
		Header: lipgloss.NewStyle().
			Bold(true).
			Foreground(pal.Primary).
			Background(pal.Background).
			Padding(0, 1),
		Title: lipgloss.NewStyle().
			Bold(true).
			Foreground(pal.Primary),
		Subtitle: lipgloss.NewStyle().
			Foreground(pal.Muted),
		cursorStyle: lipgloss.NewStyle().Bold(true).Foreground(pal.Primary),
		Panel:       panel,
		RowHead:     lipgloss.NewStyle().Bold(true).Foreground(pal.Accent),
		Selected:    lipgloss.NewStyle().Bold(true).Foreground(pal.Primary).Background(pal.Background),
		Muted:       lipgloss.NewStyle().Foreground(pal.Muted),
		Accent:      lipgloss.NewStyle().Foreground(pal.Accent),
		Success:     lipgloss.NewStyle().Foreground(pal.Success),
		Warning:     lipgloss.NewStyle().Foreground(pal.Warning),
		Danger:      lipgloss.NewStyle().Foreground(pal.Danger),
		Footer: lipgloss.NewStyle().
			Foreground(pal.Muted).
			Background(pal.Background).
			Padding(0, 1),
		Status: lipgloss.NewStyle().Foreground(pal.Muted),
	}
}
