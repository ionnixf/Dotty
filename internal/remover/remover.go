// Package remover uninstalls packages: it removes the symlink Dotty created
// and forgets the record. It never deletes repository contents or any file
// the user may have placed at the target.
package remover

import (
	"errors"
	"fmt"
	"os"

	"github.com/ion/dotty/internal/config"
	"github.com/ion/dotty/internal/storage"
)

// ErrNotASymlink is returned when the target path exists but is not a symlink,
// which means it is not something Dotty owns and should not be touched.
var ErrNotASymlink = errors.New("target is not a symlink; refusing to remove")

// ErrNotFound is returned when the package is not in the database.
var ErrNotFound = errors.New("package is not installed")

// Remover uninstalls packages.
type Remover struct {
	store *storage.Store
	paths config.Paths
	home  string
}

// New returns a Remover.
func New(store *storage.Store, paths config.Paths, home string) *Remover {
	return &Remover{store: store, paths: paths, home: home}
}

// Remove unlinks the symlink for name and drops its record. If the symlink is
// already gone, the record is still removed (idempotent).
func (r *Remover) Remove(name string) error {
	rec, ok, err := r.store.Find(name)
	if err != nil {
		return err
	}
	if !ok {
		return ErrNotFound
	}

	target, err := config.ExpandInHome(rec.Target, r.home)
	if err != nil {
		return fmt.Errorf("resolve target %q: %w", rec.Target, err)
	}

	if err := r.unlink(target); err != nil {
		return err
	}

	if err := r.store.Remove(name); err != nil {
		return fmt.Errorf("forget record for %q: %w", name, err)
	}
	return nil
}

// unlink removes target if and only if it is a symlink. Anything else — a real
// directory, a regular file, or absence — is handled defensively.
func (r *Remover) unlink(target string) error {
	info, err := os.Lstat(target)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // already gone; idempotent success
		}
		return fmt.Errorf("inspect %s: %w", target, err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		return fmt.Errorf("%w: %s", ErrNotASymlink, target)
	}
	if err := os.Remove(target); err != nil {
		return fmt.Errorf("remove symlink %s: %w", target, err)
	}
	return nil
}
