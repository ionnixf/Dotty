// Package config manages Dotty's user configuration and resolves the
// well-known filesystem locations the application reads from and writes to.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Dirs returns the canonical Dotty directories, derived from the user's
// home directory and the XDG base directory specification. All paths are
// absolute; missing directories are created on demand via Ensure.
type Paths struct {
	// ConfigDir holds configuration: ~/.config/dotty
	ConfigDir string
	// DataDir holds runtime data: ~/.local/share/dotty
	DataDir string
	// RepoDir holds cloned repositories: <DataDir>/repos
	RepoDir string
}

// ConfigFile is the absolute path to the user's settings file.
func (p Paths) ConfigFile() string { return filepath.Join(p.ConfigDir, "config.json") }

// CatalogOverride is an optional user-provided package catalog that, when
// present, takes precedence over the embedded default catalog.
func (p Paths) CatalogOverride() string { return filepath.Join(p.ConfigDir, "packages.json") }

// RepositoriesFile is the absolute path to the registry of package
// repositories the user has added. See internal/repo.Manager.
func (p Paths) RepositoriesFile() string { return filepath.Join(p.ConfigDir, "repositories.json") }

// RepoCacheDir is the directory where remote (git) repository indexes are
// cloned so their packages.json can be read locally.
func (p Paths) RepoCacheDir() string { return filepath.Join(p.DataDir, "repo-cache") }

// InstalledFile is the absolute path to the installed-package database.
func (p Paths) InstalledFile() string { return filepath.Join(p.DataDir, "installed.json") }

// DefaultPaths builds Paths from environment variables. home overrides the
// user home (used by tests); pass os.UserHomeDir in production.
func DefaultPaths(home string) (Paths, error) {
	if home == "" {
		var err error
		home, err = os.UserHomeDir()
		if err != nil {
			return Paths{}, fmt.Errorf("determine home directory: %w", err)
		}
	}

	configDir := xdg("XDG_CONFIG_HOME", filepath.Join(home, ".config"), "dotty")
	dataDir := xdg("XDG_DATA_HOME", filepath.Join(home, ".local", "share"), "dotty")

	return Paths{
		ConfigDir: configDir,
		DataDir:   dataDir,
		RepoDir:   filepath.Join(dataDir, "repos"),
	}, nil
}

// xdg resolves an XDG directory: $env if set and absolute, otherwise
// default/app, and ensures the result is absolute.
func xdg(env, def, app string) string {
	if v := os.Getenv(env); v != "" && filepath.IsAbs(v) {
		return filepath.Join(v, app)
	}
	return filepath.Join(def, app)
}

// Ensure creates the configuration, data, repo, and repo-cache directories
// with mode 0o755 if they do not already exist.
func (p Paths) Ensure() error {
	for _, dir := range []string{p.ConfigDir, p.DataDir, p.RepoDir, p.RepoCacheDir()} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create directory %s: %w", dir, err)
		}
	}
	return nil
}

// Expand converts a path that may start with "~" into an absolute path.
// A bare "~" or "~/..." is replaced with home; "~user" is left untouched
// because Go has no portable way to look up other users' homes.
func Expand(path, home string) (string, error) {
	if path == "" {
		return "", nil
	}
	if !strings.HasPrefix(path, "~") {
		return path, nil
	}
	if path == "~" {
		return home, nil
	}
	if strings.HasPrefix(path, "~/") {
		return filepath.Join(home, path[2:]), nil
	}
	// "~user/..." - leave as-is.
	return path, nil
}

// Shorten is the inverse of Expand for display: an absolute path inside home
// is rendered as "~". It does not change paths outside home.
func Shorten(path, home string) string {
	if home == "" || path == home {
		if path == home {
			return "~"
		}
		return path
	}
	rel := strings.TrimPrefix(path, home+string(filepath.Separator))
	if rel == path {
		return path
	}
	return "~" + string(filepath.Separator) + rel
}
