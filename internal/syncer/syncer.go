// Package syncer reconciles what the database claims is installed with what
// actually exists on disk. It reports problems and can repair them by
// recreating symlinks and forgetting records whose backing repo is gone.
package syncer

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/ion/dotty/internal/config"
	"github.com/ion/dotty/internal/git"
	"github.com/ion/dotty/internal/storage"
)

// ProblemKind classifies a discrepancy found during sync.
type ProblemKind int

const (
	ProblemMissingSymlink ProblemKind = iota // target is gone or not a symlink
	ProblemBrokenSymlink                     // symlink exists but points nowhere
	ProblemMissingRepo                       // repo directory missing / not a repo
)

// Problem describes one discrepancy for one record.
type Problem struct {
	Record storage.Record
	Kind   ProblemKind
	// Detail is a human-readable explanation of the specific issue.
	Detail string
}

// Syncer checks and repairs installed-package state.
type Syncer struct {
	store *storage.Store
	paths config.Paths
	home  string
	git   *git.Client
}

// New returns a Syncer.
func New(store *storage.Store, paths config.Paths, home string, g *git.Client) *Syncer {
	return &Syncer{store: store, paths: paths, home: home, git: g}
}

// Check scans installed packages and returns every problem found, in the
// order the records appear in the database.
func (s *Syncer) Check() ([]Problem, error) {
	records, err := s.store.All()
	if err != nil {
		return nil, err
	}
	var problems []Problem
	for _, rec := range records {
		p, ok := s.inspect(rec)
		if ok {
			problems = append(problems, p)
		}
	}
	return problems, nil
}

// inspect returns (problem, true) if a record has an issue, or (_, false)
// if the record is healthy. A single record may have multiple issues; we
// report the first we notice, ordered from most actionable to least.
func (s *Syncer) inspect(rec storage.Record) (Problem, bool) {
	target, err := config.Expand(rec.Target, s.home)
	if err != nil {
		return Problem{Record: rec, Kind: ProblemMissingSymlink, Detail: fmt.Sprintf("invalid target %q: %v", rec.Target, err)}, true
	}

	info, err := os.Lstat(target)
	if err != nil {
		if os.IsNotExist(err) {
			return Problem{Record: rec, Kind: ProblemMissingSymlink, Detail: fmt.Sprintf("symlink missing: %s", target)}, true
		}
		return Problem{Record: rec, Kind: ProblemMissingSymlink, Detail: fmt.Sprintf("cannot inspect %s: %v", target, err)}, true
	}
	if info.Mode()&os.ModeSymlink == 0 {
		return Problem{Record: rec, Kind: ProblemMissingSymlink, Detail: fmt.Sprintf("target is not a symlink: %s", target)}, true
	}
	// Symlink exists — does it resolve to a real directory?
	if _, err := os.Stat(target); err != nil {
		return Problem{Record: rec, Kind: ProblemBrokenSymlink, Detail: fmt.Sprintf("broken symlink: %s", target)}, true
	}

	// Symlink is healthy; verify the backing repo still exists.
	repoDir := filepath.Join(s.paths.RepoDir, rec.Name)
	isRepo, err := s.git.IsRepo(repoDir)
	if err != nil {
		return Problem{Record: rec, Kind: ProblemMissingRepo, Detail: fmt.Sprintf("cannot check repo %s: %v", repoDir, err)}, true
	}
	if !isRepo {
		return Problem{Record: rec, Kind: ProblemMissingRepo, Detail: fmt.Sprintf("repository missing: %s", repoDir)}, true
	}

	return Problem{}, false
}

// KindLabel renders a human-readable name for a ProblemKind.
func KindLabel(k ProblemKind) string {
	switch k {
	case ProblemMissingSymlink:
		return "Missing symlink"
	case ProblemBrokenSymlink:
		return "Broken symlink"
	case ProblemMissingRepo:
		return "Missing repository"
	default:
		return "Unknown problem"
	}
}

// Repair re-creates the symlink for a record if the backing repo still
// exists. It does nothing destructive: it only calls os.Symlink at the
// recorded target after removing a pre-existing broken symlink there.
func (s *Syncer) Repair(rec storage.Record) error {
	target, err := config.Expand(rec.Target, s.home)
	if err != nil {
		return fmt.Errorf("expand target %q: %w", rec.Target, err)
	}
	source := filepath.Join(s.paths.RepoDir, rec.Name, rec.Source)
	if _, err := os.Stat(source); err != nil {
		return fmt.Errorf("source missing, cannot repair: %w", err)
	}

	// Remove whatever is at target if it is a (likely broken) symlink.
	if info, err := os.Lstat(target); err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			if err := os.Remove(target); err != nil {
				return fmt.Errorf("remove broken symlink %s: %w", target, err)
			}
		} else {
			return fmt.Errorf("cannot repair: %s is a real file or directory", target)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("inspect %s: %w", target, err)
	}

	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return fmt.Errorf("create parent of %s: %w", target, err)
	}
	if err := os.Symlink(source, target); err != nil {
		return fmt.Errorf("recreate symlink %s -> %s: %w", target, source, err)
	}
	return nil
}
