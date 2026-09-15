// Package git holds the git operations drudge depends on, as a port the
// domain layer calls and an adapter carries out.
package git

import (
	"errors"
	"time"
)

// ErrNoDefaultBranch means a repository has no origin/HEAD to read a default
// branch from.
var ErrNoDefaultBranch = errors.New("no origin/HEAD is set")

// Timeouts caps how long each kind of git command may run. A command that
// outruns its cap is killed. Git has no timeout of its own.
type Timeouts struct {
	Fetch    time.Duration
	Worktree time.Duration
	Command  time.Duration
}

// Operations are the git commands drudge runs.
type Operations interface {
	// IsRepositoryRoot reports whether dir is the root of a git work tree.
	IsRepositoryRoot(dir string) (bool, error)
	// DefaultBranch returns the branch origin/HEAD points at, without the
	// remote prefix. A repository that has no such ref returns
	// ErrNoDefaultBranch.
	DefaultBranch(dir string) (string, error)
}
