// Package git wraps the system git binary for the few operations Dotty needs:
// clone, pull, and detecting whether a directory is a git working tree. It
// captures combined stdout/stderr so the TUI can surface real git errors.
package git

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ErrNotFound is returned when the git binary is not on PATH. Callers can
// surface a friendly message rather than a generic exec failure.
var ErrNotFound = errors.New("git executable not found in PATH")

// Client runs git commands. All methods are safe to call from a tea.Cmd;
// they never touch the terminal directly.
type Client struct{}

// New returns a Client.
func New() *Client { return &Client{} }

// ensure verifies the git binary exists, returning ErrNotFound if not.
func (c *Client) ensure() error {
	if _, err := exec.LookPath("git"); err != nil {
		return ErrNotFound
	}
	return nil
}

// Clone clones repo into dst. If dst already exists and is non-empty the
// caller is expected to have handled that; Clone will fail via git itself.
func (c *Client) Clone(repo, dst string) (string, error) {
	if err := c.ensure(); err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return "", fmt.Errorf("create parent of %s: %w", dst, err)
	}
	out, err := c.run(dst, "clone", "--depth", "1", repo, dst)
	if err != nil {
		return out, fmt.Errorf("git clone %s: %w", repo, err)
	}
	return out, nil
}

// Pull runs git pull inside repoDir and returns git's combined output.
func (c *Client) Pull(repoDir string) (string, error) {
	if err := c.ensure(); err != nil {
		return "", err
	}
	out, err := c.run(repoDir, "pull", "--ff-only")
	if err != nil {
		return out, fmt.Errorf("git pull in %s: %w", repoDir, err)
	}
	return out, nil
}

// IsRepo reports whether dir is inside a git working tree.
func (c *Client) IsRepo(dir string) (bool, error) {
	if err := c.ensure(); err != nil {
		return false, err
	}
	if _, err := os.Stat(dir); err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	_, err := c.run(dir, "rev-parse", "--is-inside-work-tree")
	if err != nil {
		// Not a repo, or git error; treat as not-a-repo for sync purposes.
		return false, nil
	}
	return true, nil
}

// run executes git with the given args inside dir and returns trimmed
// combined output. dir may be "" to run in the current directory.
func (c *Client) run(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	out, err := cmd.CombinedOutput()
	body := strings.TrimRight(string(out), "\n\r")
	return body, err
}
