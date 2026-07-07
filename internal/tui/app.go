package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// App is the root Bubble Tea model. It owns the shared deps, instantiates every
// screen once, tracks the active screen, and renders the chrome (header bar,
// bordered content panel, footer help line) around the active screen's body.
//
// Navigation is message-driven: screens emit navMsg / backMsg / quitMsg and
// the App reacts to them here, so screens never reference each other directly.
type App struct {
	deps *deps

	screens map[screenKind]screen
	active  screenKind
	current screen

	width  int
	height int
}

// NewApp builds the root model from the shared deps. It constructs each screen
// exactly once; switching screens just changes which one is active and calls
// its enter() so it can refresh its data.
func NewApp(d *deps) *App {
	screens := map[screenKind]screen{
		screenMenu:      newMenuScreen(d),
		screenInstall:   newInstallScreen(d),
		screenUpdate:    newUpdateScreen(d),
		screenRemove:    newRemoveScreen(d),
		screenInstalled: newInstalledScreen(d),
		screenSync:      newSyncScreen(d),
		screenImport:    newImportScreen(d),
		screenSettings:  newSettingsScreen(d),
	}
	app := &App{deps: d, screens: screens, active: screenMenu, current: screens[screenMenu]}
	app.current.enter()
	return app
}

// Init starts nothing: screens kick off their own async work (spinners, scans)
// from their enter() call or on first key press.
func (a *App) Init() tea.Cmd { return nil }

// Update forwards messages to the active screen and intercepts the navigation
// messages it returns. WindowSizeMsg is handled here so every screen gets the
// dimensions without each one subscribing individually.
func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width, a.height = msg.Width, msg.Height
		for _, s := range a.screens {
			s.setSize(a.width, a.height)
		}
		return a, nil

	case navMsg:
		return a.switchTo(msg.to)

	case backMsg:
		// Every non-menu screen returns to the menu.
		if a.active != screenMenu {
			return a.switchTo(screenMenu)
		}
		return a, nil

	case quitMsg:
		return a, tea.Quit
	}

	// Delegate to the active screen.
	next, cmd := a.current.Update(msg)
	if sc, ok := next.(screen); ok {
		a.current = sc
	}
	return a, cmd
}

// switchTo activates a screen, calling enter() to let it refresh, and returns
// any command the screen wants to run immediately (e.g. an initial scan).
func (a *App) switchTo(kind screenKind) (tea.Model, tea.Cmd) {
	target, ok := a.screens[kind]
	if !ok {
		// Unknown destination: fall back to the menu rather than crashing.
		target = a.screens[screenMenu]
		kind = screenMenu
	}
	a.active = kind
	a.current = target
	target.setSize(a.width, a.height)
	target.enter()
	// Re-run Init so any screen that schedules work in Init gets a chance.
	return a, target.Init()
}

// View lays out the header, the active screen inside a bordered panel, and the
// footer. When the terminal has not reported a size yet we fall back to 80x24
// so the first frame is not empty.
func (a *App) View() string {
	width := a.width
	if width == 0 {
		width = 80
	}

	header := a.renderHeader(width)
	footer := a.renderFooter(width)
	body := a.current.View()

	// Wrap the body in the panel, constraining it to the available width.
	panel := a.deps.theme.Panel.Width(max(0, width-4)).Render
	paneled := panel(body)

	return lipgloss.JoinVertical(lipgloss.Left, header, paneled, footer)
}

// renderHeader draws the title bar: "Dotty › <screen>" on the left and the
// current theme tag on the right, padded to the full width.
func (a *App) renderHeader(width int) string {
	left := "Dotty"
	if a.current != nil && a.current.title() != "" && a.current.title() != "Dotty" {
		left = "Dotty  ›  " + a.current.title()
	}
	right := "theme: " + a.deps.theme.Name

	// Render once with the header style, then pad between the two halves and
	// let Width enforce the final terminal width.
	leftRendered := a.deps.theme.Header.Render(left)
	rightRendered := a.deps.theme.Header.Render(right)

	gap := width - lipgloss.Width(leftRendered) - lipgloss.Width(rightRendered)
	if gap < 0 {
		gap = 0
	}
	bar := leftRendered + strings.Repeat(" ", gap) + rightRendered
	return a.deps.theme.Header.Width(width).Render(bar)
}

// renderFooter draws the help line for the active screen, padded to the width.
func (a *App) renderFooter(width int) string {
	help := ""
	if a.current != nil {
		help = a.current.help()
	}
	return a.deps.theme.Footer.Width(width).Render(help)
}
