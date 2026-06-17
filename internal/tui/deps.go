package tui

import (
	"github.com/ion/dotty/internal/catalog"
	"github.com/ion/dotty/internal/config"
	"github.com/ion/dotty/internal/git"
	"github.com/ion/dotty/internal/installer"
	"github.com/ion/dotty/internal/remover"
	"github.com/ion/dotty/internal/storage"
	"github.com/ion/dotty/internal/syncer"
	"github.com/ion/dotty/internal/updater"
)

// deps is the bag of dependencies every screen needs. It is constructed once
// in main and passed by pointer to each screen, so actions see the latest
// state (e.g. a freshly saved settings object) without global variables.
type deps struct {
	paths     config.Paths
	home      string
	configDir string // user's ~/.config, used by the scanner

	store     *storage.Store
	git       *git.Client
	installer *installer.Installer
	updater   *updater.Updater
	remover   *remover.Remover
	syncer    *syncer.Syncer

	// mutable settings + theme live behind the same deps pointer so the
	// Settings screen can update them and every other screen picks the
	// change up on the next render.
	settings *config.Settings
	theme    *Theme
}

// reloadCatalog returns the effective package catalog, honouring the user's
// optional override file. Errors propagate to the caller for display.
func (d *deps) reloadCatalog() ([]catalog.Package, error) {
	return catalog.Load(d.paths.CatalogOverride())
}

// installedRecords returns the current installed-package database.
func (d *deps) installedRecords() ([]storage.Record, error) {
	return d.store.All()
}
