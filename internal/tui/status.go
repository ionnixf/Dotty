package tui

import (
	"os"

	"github.com/ion/dotty/internal/config"
	"github.com/ion/dotty/internal/storage"
)

// liveStatus inspects the on-disk symlink for a record and returns a short,
// human-readable status label for the Installed table:
//
//   - "Installed": target is a symlink that resolves to an existing directory.
//   - "Broken":    target is a symlink but its destination is missing.
//   - "Missing":   target is absent or is not a symlink at all.
func liveStatus(d *deps, rec storage.Record) string {
	target, err := config.Expand(rec.Target, d.home)
	if err != nil {
		return "Missing"
	}
	info, err := os.Lstat(target)
	if err != nil {
		return "Missing"
	}
	if info.Mode()&os.ModeSymlink == 0 {
		return "Missing"
	}
	if _, err := os.Stat(target); err != nil {
		return "Broken"
	}
	return "Installed"
}
