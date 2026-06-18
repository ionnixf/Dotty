// Command dotty is a minimalist dotfiles manager with a terminal UI.
//
// It clones config repositories, symlinks them into the user's config tree,
// tracks what is installed in a local JSON database, and lets the user update,
// remove, re-sync, import existing configs, and tweak settings — all from a
// keyboard-driven Bubble Tea interface inspired by lazygit / lazydocker / k9s.
package main

import (
	"fmt"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/ion/dotty/internal/config"
	"github.com/ion/dotty/internal/tui"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "dotty:", err)
		os.Exit(1)
	}
}

// run is split out of main so error handling lives in one place. It performs
// every step that can fail before handing control to Bubble Tea.
func run() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("determine home directory: %w", err)
	}

	paths, err := config.DefaultPaths(home)
	if err != nil {
		return err
	}
	if err := paths.Ensure(); err != nil {
		return err
	}

	// The Import screen scans the user's real ~/.config, so we resolve it the
	// same way config.DefaultPaths does (honouring XDG_CONFIG_HOME).
	userConfigDir := os.Getenv("XDG_CONFIG_HOME")
	if userConfigDir == "" || !filepath.IsAbs(userConfigDir) {
		userConfigDir = filepath.Join(home, ".config")
	}

	deps, err := tui.NewDeps(paths, home, userConfigDir)
	if err != nil {
		return err
	}

	// WithAltScreen gives a full-screen, restorable-on-exit experience like
	// lazygit. We also enable mouse so future row selection works unchanged.
	p := tea.NewProgram(tui.NewApp(deps), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("run UI: %w", err)
	}
	return nil
}
