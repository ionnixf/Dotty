// Package installer clones a package repository and links one of its
// subdirectories into the user's config tree. It is deliberately stateless:
// it receives its dependencies per call and persists results via storage.Store.
package installer

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ion/dotty/internal/config"
	"github.com/ion/dotty/internal/git"
	"github.com/ion/dotty/internal/storage"
)

// ErrTargetExists is returned when the symlink target already exists and is
// not a symlink under Dotty's control. The remover never deletes user files,
// so installation must stop and ask the user instead of clobbering data.
var ErrTargetExists = errors.New("target path already exists; refusing to overwrite")

// Installer performs package installation.
type Installer struct {
	git   *git.Client
	store *storage.Store
	paths config.Paths
	home  string
}

// New returns an Installer.
func New(g *git.Client, store *storage.Store, paths config.Paths, home string) *Installer {
	return &Installer{git: g, store: store, paths: paths, home: home}
}

// Request captures the inputs for one install operation.
type Request struct {
	Name   string
	Repo   string
	Source string // directory inside the repo to link from
	Target string // destination path (may contain "~")
}

// Result is what Install returns on success.
type Result struct {
	Name    string
	Repo    string
	Source  string
	Target  string // absolute, expanded
	RepoDir string // absolute path the repo was cloned to
}

// Install clones (or reuses) the repo and creates the symlink from
// repoDir/Source to Target. On success the package is recorded.
func (i *Installer) Install(req Request) (Result, error) {
	if req.Name == "" {
		return Result{}, errors.New("empty package name")
	}

	repoDir := filepath.Join(i.paths.RepoDir, req.Name)
	target, err := config.Expand(req.Target, i.home)
	if err != nil {
		return Result{}, fmt.Errorf("expand target %q: %w", req.Target, err)
	}

	if err := i.ensureRepo(req.Repo, repoDir); err != nil {
		return Result{}, err
	}

	source := filepath.Join(repoDir, req.Source)
	if err := i.link(source, target); err != nil {
		return Result{}, err
	}

	rec := storage.Record{
		Name:   req.Name,
		Repo:   req.Repo,
		Source: req.Source,
		Target: req.Target,
	}
	if err := i.store.Add(rec); err != nil {
		return Result{}, fmt.Errorf("record install of %q: %w", req.Name, err)
	}

	return Result{
		Name:    req.Name,
		Repo:    req.Repo,
		Source:  req.Source,
		Target:  target,
		RepoDir: repoDir,
	}, nil
}

// ensureRepo clones repo into repoDir if it is not already a git repo.
// If the directory exists but is not a repo, it is an unexpected state and
// we refuse rather than guess. On clone failure any partially-created repoDir
// is removed so a retry is not permanently blocked by an empty directory git
// may have left behind.
func (i *Installer) ensureRepo(repo, repoDir string) error {
	isRepo, err := i.git.IsRepo(repoDir)
	if err != nil {
		return fmt.Errorf("check repo %s: %w", repoDir, err)
	}
	if isRepo {
		return nil
	}
	if _, err := os.Stat(repoDir); err == nil {
		// Directory exists but is not a git repo. Refuse to avoid deleting
		// anything we do not own.
		return fmt.Errorf("repo directory %s exists but is not a git repository", repoDir)
	}
	if _, err := i.git.Clone(repo, repoDir); err != nil {
		// git may create repoDir before failing (e.g. a non-existent remote).
		// Clean up an empty leftover so the next attempt can clone cleanly;
		// only remove it if it is still empty to stay strictly non-destructive.
		removeIfEmpty(repoDir)
		return err
	}
	return nil
}

// removeIfEmpty deletes dir only when it contains no entries, so we never
// erase anything git or the user placed there.
func removeIfEmpty(dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	if len(entries) != 0 {
		return
	}
	_ = os.Remove(dir)
}

// link creates a symlink source -> target (i.e. target points to source).
// If target is already a symlink it is replaced (idempotent re-install). If
// target is any other existing filesystem entry, ErrTargetExists is returned.
func (i *Installer) link(source, target string) error {
	if _, err := os.Stat(source); err != nil {
		return fmt.Errorf("source %s missing in repo: %w", source, err)
	}

	if info, err := os.Lstat(target); err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			// Existing symlink: safe to replace.
			if err := os.Remove(target); err != nil {
				return fmt.Errorf("remove existing symlink %s: %w", target, err)
			}
		} else {
			return fmt.Errorf("%w: %s", ErrTargetExists, target)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("inspect target %s: %w", target, err)
	}

	parent := filepath.Dir(target)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return fmt.Errorf("create parent of %s: %w", target, err)
	}
	if err := os.Symlink(source, target); err != nil {
		return fmt.Errorf("symlink %s -> %s: %w", target, source, err)
	}
	return nil
}
