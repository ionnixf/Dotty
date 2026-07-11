// Package installer clones a package repository and links one of its
// subdirectories into the user's config tree. It is deliberately stateless:
// it receives its dependencies per call and persists results via storage.Store.
package installer

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

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
//
// An empty Source links the whole repository root to Target ("target -> repo
// contents"), which is the common case for dotfiles repos that keep their
// config at the root. A non-empty Source links only that subdirectory.
func (i *Installer) Install(req Request) (Result, error) {
	if req.Name == "" {
		return Result{}, errors.New("empty package name")
	}
	if req.Repo == "" {
		return Result{}, errors.New("empty repo URL")
	}

	repoDir, err := config.SafeJoin(i.paths.RepoDir, req.Name)
	if err != nil {
		return Result{}, fmt.Errorf("invalid package name: %w", err)
	}
	target, err := config.ExpandInHome(req.Target, i.home)
	if err != nil {
		return Result{}, fmt.Errorf("resolve target %q: %w", req.Target, err)
	}

	source, err := LinkSource(repoDir, req.Source)
	if err != nil {
		return Result{}, err
	}
	// Check the destination before changing the cached repository. A failed
	// install must not replace a working repository just to discover that its
	// target belongs to a different configuration.
	if err := checkTarget(source, target); err != nil {
		return Result{}, err
	}

	if err := i.ensureRepo(req.Repo, repoDir); err != nil {
		return Result{}, err
	}
	created, err := i.link(source, target)
	if err != nil {
		return Result{}, err
	}

	rec := storage.Record{
		Name:   req.Name,
		Repo:   req.Repo,
		Source: req.Source,
		Target: req.Target,
	}
	if err := i.store.Add(rec); err != nil {
		if created {
			_ = os.Remove(target)
		}
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

// LinkSource returns the absolute path within repoDir that should be linked
// to the target. An empty source means "the repository root" — the common
// case for dotfiles repos that keep their config at the root. This is the
// single source of truth for that convention; the syncer reuses it so install
// and repair always agree on what gets linked.
func LinkSource(repoDir, source string) (string, error) {
	if source == "" {
		return repoDir, nil
	}
	if filepath.IsAbs(source) || strings.Contains(source, "..") || strings.Contains(source, "\\") {
		return "", fmt.Errorf("invalid source path %q: absolute or relative parent paths not allowed", source)
	}
	path := filepath.Join(repoDir, source)
	rel, err := filepath.Rel(repoDir, path)
	if err != nil || strings.HasPrefix(rel, "..") || filepath.IsAbs(rel) {
		return "", fmt.Errorf("invalid source path %q: escapes repository directory", source)
	}
	return path, nil
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
		remote, err := i.git.RemoteURL(repoDir)
		if err == nil && normalizeURL(remote) == normalizeURL(repo) {
			return nil
		}
		// Remote URL differs (e.g. user selected a different configuration alternative):
		// clear the old repo directory so a clean clone can be fetched.
		if err := os.RemoveAll(repoDir); err != nil {
			return fmt.Errorf("clear old repository %s for new URL: %w", repoDir, err)
		}
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

func normalizeURL(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimSuffix(s, "/")
	s = strings.TrimSuffix(s, ".git")
	return strings.ToLower(s)
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

// link creates a symlink at target pointing to source (target -> source).
// It returns (created=true, nil) if a new symlink was created, or (created=false, nil)
// if a correct symlink was already in place (no-op).
func (i *Installer) link(source, target string) (bool, error) {
	if _, err := os.Stat(source); err != nil {
		return false, fmt.Errorf("link source %s missing in repo: %w", source, err)
	}
	// Dotty links directories or files. Source must exist.

	if err := checkTarget(source, target); err != nil {
		return false, err
	}
	if _, err := os.Lstat(target); err == nil {
		// checkTarget established that this is the correct existing symlink.
		return false, nil
	}

	parent := filepath.Dir(target)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return false, fmt.Errorf("create parent of %s: %w", target, err)
	}
	if err := os.Symlink(source, target); err != nil {
		return false, fmt.Errorf("symlink %s -> %s: %w", target, source, err)
	}
	return true, nil
}

// checkTarget verifies that target is either absent or already points at source.
// It intentionally does not modify the filesystem so callers can run it before
// changing a repository.
func checkTarget(source, target string) error {
	existing, err := os.Lstat(target)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("inspect target %s: %w", target, err)
	}
	if existing.Mode()&os.ModeSymlink == 0 {
		return fmt.Errorf("%w: %s", ErrTargetExists, target)
	}
	existingPath, err := os.Readlink(target)
	if err != nil || existingPath != source {
		return fmt.Errorf("%w: %s", ErrTargetExists, target)
	}
	return nil
}
