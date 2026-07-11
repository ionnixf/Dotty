package tui

import (
	"fmt"
	"os"

	"github.com/ion/dotty/internal/catalog"
	"github.com/ion/dotty/internal/config"
	"github.com/ion/dotty/internal/git"
	"github.com/ion/dotty/internal/installer"
	"github.com/ion/dotty/internal/remover"
	"github.com/ion/dotty/internal/repo"
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
	manager   *repo.Manager
	resolver  *repo.Resolver
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

// NewDeps wires the real domain services into a deps bag for the TUI. It is
// the single place that constructs the store, git client, and the
// installer/updater/remover/syncer services, so main stays trivial and there
// are no globals. settingsPath/configDir/home are accepted explicitly so they
// can be varied in tests later.
func NewDeps(paths config.Paths, home, userConfigDir string) (*deps, error) {
	settings, err := config.LoadSettings(paths)
	if err != nil {
		return nil, err
	}
	settings.Theme = config.NormaliseTheme(settings.Theme)
	theme := NewTheme(settings.Theme)

	paths.RepoDir = config.ResolveRepoDir(settings, paths)

	store := storage.New(paths.InstalledFile())
	gc := git.New()

	manager, err := repo.NewManager(paths.RepositoriesFile())
	if err != nil {
		return nil, fmt.Errorf("load repositories: %w", err)
	}
	resolver := repo.NewResolver(manager, gc, paths)

	return &deps{
		paths:     paths,
		home:      home,
		configDir: userConfigDir,
		store:     store,
		git:       gc,
		manager:   manager,
		resolver:  resolver,
		installer: installer.New(gc, store, paths, home),
		updater:   updater.New(gc, store, paths),
		remover:   remover.New(store, paths, home),
		syncer:    syncer.New(store, paths, home, gc),
		settings:  &settings,
		theme:     &theme,
	}, nil
}

// updateSettings applies the new settings dynamically to all services.
func (d *deps) updateSettings(s config.Settings) {
	*d.settings = s
	d.paths.RepoDir = config.ResolveRepoDir(s, d.paths)

	// Recreate services using the updated paths
	d.resolver = repo.NewResolver(d.manager, d.git, d.paths)
	d.installer = installer.New(d.git, d.store, d.paths, d.home)
	d.updater = updater.New(d.git, d.store, d.paths)
	d.remover = remover.New(d.store, d.paths, d.home)
	d.syncer = syncer.New(d.store, d.paths, d.home, d.git)
}

// reloadCatalog returns the effective package catalog.
//
// It prefers the resolver, which merges every registered repository (the
// always-seeded "official" repository contributes the embedded catalog) into a
// single view. If the user has a legacy packages.json override in the config
// directory, that file wins outright so existing setups keep working: the
// override was the pre-repository mechanism for customising the catalog and is
// preserved verbatim.
func (d *deps) reloadCatalog() ([]catalog.Package, error) {
	if override := d.paths.CatalogOverride(); override != "" {
		if _, err := os.Stat(override); err == nil {
			if pkgs, err := catalog.Load(override); err == nil {
				return pkgs, nil
			} else {
				return nil, err
			}
		} else if !os.IsNotExist(err) {
			return nil, err
		}
	}
	return d.resolver.All()
}

// installedRecords returns the current installed-package database.
func (d *deps) installedRecords() ([]storage.Record, error) {
	return d.store.All()
}
