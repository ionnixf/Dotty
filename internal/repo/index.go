// Package repo adds a repository abstraction on top of the catalog: users can
// register multiple package sources (local directories or git URLs), and a
// resolver merges them into a single view with deterministic conflict
// resolution. The installer consumes catalog.Package exactly as before, so the
// install workflow and TUI are unchanged.
package repo

import "github.com/ion/dotty/internal/catalog"

// Index is the normalized, taggable view of one repository's packages. It is
// the unit a Resolver merges. Each entry keeps a reference to the repository
// name it came from so callers can report provenance and the installer can be
// driven deterministically.
type Index struct {
	// RepoName is the repository this index was built from.
	RepoName string
	// Packages is that repository's package list.
	Packages []catalog.Package
}

// Entry is a single package together with the repository it was resolved from.
// Resolver.All returns a slice of Entry in the same order as Index.Packages.
type Entry struct {
	catalog.Package
	// RepoName is the repository that contributed this package after conflict
	// resolution.
	RepoName string
}
