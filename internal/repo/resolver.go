package repo

import (
	"fmt"

	"github.com/ion/dotty/internal/catalog"
	"github.com/ion/dotty/internal/config"
	"github.com/ion/dotty/internal/git"
)

// Resolver merges the indexes of all registered repositories into a single
// view and resolves a package name to one source. Conflict resolution is
// deterministic: the first-registered repository that provides a name wins.
type Resolver struct {
	manager  *Manager
	git      *git.Client
	paths    config.Paths
	embedded []catalog.Package // parsed official catalog, cached for speed
}

// NewResolver builds a Resolver over the given manager. embedded is the parsed
// official catalog used when the official repository is present; pass nil to
// have it parsed lazily from the embedded bytes.
func NewResolver(m *Manager, g *git.Client, paths config.Paths) *Resolver {
	return &Resolver{manager: m, git: g, paths: paths}
}

// officialIndex builds the index for the embedded official repository without
// any disk or network IO.
func (r *Resolver) officialIndex() (Index, error) {
	pkgs, err := catalog.Parse(EmbeddedPackagesJSON())
	if err != nil {
		return Index{}, fmt.Errorf("parse embedded catalog: %w", err)
	}
	return Index{RepoName: OfficialRepoName, Packages: pkgs}, nil
}

// indexes fetches every repository's index in registration order. A failure in
// one repository is collected rather than aborting the whole merge: the user
// still sees packages from the repositories that did load, and the failing
// repository's error is returned for display.
func (r *Resolver) indexes() ([]Index, error) {
	repos := r.manager.List()
	out := make([]Index, 0, len(repos))
	var firstErr error
	for _, rep := range repos {
		idx, err := r.fetchIndex(rep)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		out = append(out, idx)
	}
	return out, firstErr
}

// fetchIndex builds one repository's index, dispatching on its kind. The
// embedded kind bypasses the git client and filesystem entirely.
func (r *Resolver) fetchIndex(rep Repository) (Index, error) {
	if rep.Kind == KindEmbedded {
		return r.officialIndex()
	}
	idx, err := rep.Fetch(r.git, r.paths.RepoCacheDir())
	if err != nil && rep.Name == OfficialRepoName {
		// First launch and offline operation remain possible without a cloned
		// official catalog.
		return r.officialIndex()
	}
	return idx, err
}

// Merge builds the unified index: every repository's packages, in registration
// order, with conflicts resolved first-registered-wins. The returned error, if
// any, is from a repository that failed to fetch; the partial merge is still
// returned so callers can show what did load.
func (r *Resolver) Merge() ([]Entry, error) {
	indexes, firstErr := r.indexes()

	entries := make([]Entry, 0)
	seen := make(map[string]bool)
	for _, idx := range indexes {
		for _, p := range idx.Packages {
			if seen[p.Name] {
				continue
			}
			seen[p.Name] = true
			entries = append(entries, Entry{Package: p, RepoName: idx.RepoName})
		}
	}
	return entries, firstErr
}

// All returns the merged packages sorted by name for stable display. It is the
// resolver's public read API and the natural drop-in for catalog.Load.
func (r *Resolver) All() ([]catalog.Package, error) {
	entries, err := r.Merge()
	pkgs := make([]catalog.Package, 0, len(entries))
	for _, e := range entries {
		pkgs = append(pkgs, e.Package)
	}
	return catalog.Sorted(pkgs), err
}

// Resolve picks the single package contributing to name, returning the package
// and the repository it came from. It returns ok=false if no repository
// provides name (after conflict resolution).
func (r *Resolver) Resolve(name string) (Entry, bool, error) {
	entries, err := r.Merge()
	for _, e := range entries {
		if e.Name == name {
			return e, true, err
		}
	}
	return Entry{}, false, err
}
