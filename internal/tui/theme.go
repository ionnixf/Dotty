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
		Primary:    lipgloss.Color("#a78bfa"), // soft violet
		Accent:     lipgloss.Color("#22d3ee"), // cyan
		Muted:      lipgloss.Color("#6b7280"), // slate gray
		Success:    lipgloss.Color("#34d399"), // emerald
		Warning:    lipgloss.Color("#fbbf24"), // amber
		Danger:     lipgloss.Color("#f87171"), // rose
		Border:     lipgloss.Color("#4b5563"), // dark gray border
		Background: lipgloss.Color("#1f2937"), // dark background for header/footer
	},
	"light": {
		Primary:    lipgloss.Color("#6d28d9"), // deep violet
		Accent:     lipgloss.Color("#0891b2"), // deep cyan
		Muted:      lipgloss.Color("#6b7280"), // gray
		Success:    lipgloss.Color("#059669"), // deep emerald
		Warning:    lipgloss.Color("#d97706"), // deep amber
		Danger:     lipgloss.Color("#dc2626"), // red
		Border:     lipgloss.Color("#d1d5db"), // light gray border
		Background: lipgloss.Color("#f3f4f6"), // light background for header/footer
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
		Padding(1, 2)

	return Theme{
		Name: name,
		Header: lipgloss.NewStyle().
			Bold(true).
			Foreground(pal.Primary).
			Background(pal.Background).
			Padding(0, 2),
		Title: lipgloss.NewStyle().
			Bold(true).
			Foreground(pal.Primary),
		Subtitle: lipgloss.NewStyle().
			Foreground(pal.Muted),
		cursorStyle: lipgloss.NewStyle().Bold(true).Foreground(pal.Primary),
		Panel:       panel,
		RowHead:     lipgloss.NewStyle().Bold(true).Foreground(pal.Accent).Underline(true),
		Selected:    lipgloss.NewStyle().Bold(true).Foreground(pal.Primary).Background(pal.Background).Padding(0, 1),
		Muted:       lipgloss.NewStyle().Foreground(pal.Muted),
		Accent:      lipgloss.NewStyle().Foreground(pal.Accent),
		Success:     lipgloss.NewStyle().Foreground(pal.Success),
		Warning:     lipgloss.NewStyle().Foreground(pal.Warning),
		Danger:      lipgloss.NewStyle().Foreground(pal.Danger),
		Footer: lipgloss.NewStyle().
			Foreground(pal.Muted).
			Background(pal.Background).
			Padding(0, 2),
		Status: lipgloss.NewStyle().Foreground(pal.Muted),
	}
}
