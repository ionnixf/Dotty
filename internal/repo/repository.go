package repo

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ion/dotty/internal/catalog"
	"github.com/ion/dotty/internal/git"
)

// Kind classifies how a Repository's index is fetched.
type Kind string

const (
	// KindLocal reads packages.json directly from a local directory.
	KindLocal Kind = "local"
	// KindGit clones the repository (shallow) and reads its packages.json.
	KindGit Kind = "git"
)

// Repository is one registered package source. It is JSON-serialisable so the
// Manager can persist the registry; the in-memory behaviour lives in Fetch.
type Repository struct {
	Name string `json:"name"`
	URL  string `json:"url"` // local path or git URL
	Kind Kind   `json:"kind"`
}

// inferKind decides whether url is a local directory or a git URL. A path that
// exists on disk is local; anything else is treated as a git remote.
func inferKind(url string) (Kind, error) {
	if url == "" {
		return "", fmt.Errorf("empty repository URL")
	}
	if info, err := os.Stat(url); err == nil && info.IsDir() {
		return KindLocal, nil
	}
	if isLikelyGitURL(url) {
		return KindGit, nil
	}
	return "", fmt.Errorf("cannot determine repository kind for %q (not an existing directory or a recognised git URL)", url)
}

// isLikelyGitURL reports whether s looks like a git remote: an SCP-style
// "host:path" or a "scheme://" URL. Local paths are rejected here; the caller
// checks the filesystem first.
func isLikelyGitURL(s string) bool {
	for _, scheme := range []string{"https://", "http://", "ssh://", "git://", "file://"} {
		if strings.HasPrefix(s, scheme) {
			return true
		}
	}
	// SCP style: user@host:path, with no leading slash.
	if strings.Contains(s, ":") && !strings.HasPrefix(s, "/") {
		return true
	}
	return false
}

// Fetch builds this repository's Index, reading packages.json from the
// appropriate place. For git repositories it clones into cacheDir (a fresh
// shallow clone each time, which also serves as the refresh). For local
// repositories it reads directly with no copying.
//
// packagesFile is the absolute path to the repository's packages.json when
// kind is local; for git it is the path inside the cloned tree.
func (r Repository) Fetch(g *git.Client, cacheDir string) (Index, error) {
	switch r.Kind {
	case KindLocal:
		return fetchLocal(r)
	case KindGit:
		return fetchGit(r, g, cacheDir)
	default:
		return Index{}, fmt.Errorf("repository %q: unknown kind %q", r.Name, r.Kind)
	}
}

func fetchLocal(r Repository) (Index, error) {
	info, err := os.Stat(r.URL)
	if err != nil {
		return Index{}, fmt.Errorf("repository %q: %w", r.Name, err)
	}
	if !info.IsDir() {
		return Index{}, fmt.Errorf("repository %q: %s is not a directory", r.Name, r.URL)
	}
	path := filepath.Join(r.URL, "packages.json")
	return readPackages(r.Name, path)
}

func fetchGit(r Repository, g *git.Client, cacheDir string) (Index, error) {
	if g == nil {
		return Index{}, fmt.Errorf("repository %q: git client is required for git repositories", r.Name)
	}
	dst := filepath.Join(cacheDir, r.Name)
	// A shallow clone into a clean destination keeps the cached index current.
	// If a previous clone exists, remove it so the next fetch is deterministic.
	if err := os.RemoveAll(dst); err != nil && !os.IsNotExist(err) {
		return Index{}, fmt.Errorf("repository %q: clear cache %s: %w", r.Name, dst, err)
	}
	if _, err := g.Clone(r.URL, dst); err != nil {
		return Index{}, fmt.Errorf("repository %q: %w", r.Name, err)
	}
	path := filepath.Join(dst, "packages.json")
	return readPackages(r.Name, path)
}

// readPackages loads and parses a packages.json file, returning an Index
// tagged with repoName. A missing file yields an empty index (the repository
// advertises no packages) rather than an error, so an empty repo is valid.
func readPackages(repoName, path string) (Index, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Index{RepoName: repoName, Packages: []catalog.Package{}}, nil
		}
		return Index{}, fmt.Errorf("repository %q: read %s: %w", repoName, path, err)
	}
	pkgs, err := catalog.Parse(raw)
	if err != nil {
		return Index{}, fmt.Errorf("repository %q: parse %s: %w", repoName, path, err)
	}
	return Index{RepoName: repoName, Packages: pkgs}, nil
}
