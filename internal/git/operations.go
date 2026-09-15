// Package git holds the git operations drudge depends on, as a port the
// domain layer calls and an adapter carries out.
package git

import (
	"errors"
	"time"
)

// OriginRemote is the remote a repository is fetched from. A repository that
// does not have it is worked on locally.
const OriginRemote = "origin"

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
	// HasRemote reports whether a repository has the named remote.
	HasRemote(dir string, remote string) (bool, error)
	// Fetch updates the tracking ref of one branch of a remote.
	Fetch(dir string, remote string, branch string) error
	// AddDetachedWorktree checks a repository out at ref in a new worktree at
	// path, with no branch on it. A path holding files fails.
	AddDetachedWorktree(dir string, path string, ref string) error
}
