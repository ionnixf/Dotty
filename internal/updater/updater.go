// Package updater runs git pull on installed packages.
package updater

import (
	"fmt"
	"path/filepath"

	"github.com/ion/dotty/internal/config"
	"github.com/ion/dotty/internal/git"
	"github.com/ion/dotty/internal/storage"
)

// Updater updates installed packages by pulling their repositories.
type Updater struct {
	git   *git.Client
	store *storage.Store
	paths config.Paths
}

// New returns an Updater.
func New(g *git.Client, store *storage.Store, paths config.Paths) *Updater {
	return &Updater{git: g, store: store, paths: paths}
}

// Outcome describes the result of updating a single package.
type Outcome struct {
	Name   string
	Output string
	Err    error
}

// Update pulls the repository for the named package.
func (u *Updater) Update(name string) Outcome {
	rec, ok, err := u.store.Find(name)
	if err != nil {
		return Outcome{Name: name, Err: err}
	}
	if !ok {
		return Outcome{Name: name, Err: fmt.Errorf("package %q is not installed", name)}
	}
	if rec.Repo == storage.ImportedRepoTag {
		return Outcome{Name: name, Output: "local imported configuration; skipping update"}
	}
	repoDir := filepath.Join(u.paths.RepoDir, rec.Name)
	out, err := u.git.Pull(repoDir)
	return Outcome{Name: name, Output: out, Err: err}
}

// UpdateAll pulls every installed package and returns one Outcome per
// package, preserving the database's name ordering.
func (u *Updater) UpdateAll() ([]Outcome, error) {
	records, err := u.store.All()
	if err != nil {
		return nil, err
	}
	results := make([]Outcome, 0, len(records))
	for _, rec := range records {
		if rec.Repo == storage.ImportedRepoTag {
			results = append(results, Outcome{Name: rec.Name, Output: "local imported configuration; skipping update"})
			continue
		}
		repoDir := filepath.Join(u.paths.RepoDir, rec.Name)
		out, err := u.git.Pull(repoDir)
		results = append(results, Outcome{Name: rec.Name, Output: out, Err: err})
	}
	return results, nil
}
