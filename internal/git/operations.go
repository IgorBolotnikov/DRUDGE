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
	// HasWorktree reports whether path is registered as a worktree of the
	// repository. A worktree whose directory was deleted stays registered
	// until it is pruned, so this answers what the repository knows and not
	// what is on disk.
	HasWorktree(dir string, path string) (bool, error)
	// IsDirty reports whether a work tree holds changes that are not
	// committed. Untracked files count and ignored files do not.
	IsDirty(dir string) (bool, error)
	// Stash puts the uncommitted changes of a work tree aside under a message
	// and returns the commit holding them. Untracked files go with them and
	// ignored files stay. A clean work tree returns an empty commit.
	Stash(dir string, message string) (string, error)
	// BranchExists reports whether a repository has a branch of this name.
	BranchExists(dir string, branch string) (bool, error)
	// CommitCount returns how many commits tip holds that base does not.
	CommitCount(dir string, base string, tip string) (int, error)
	// CreateBranch creates a branch at start and checks it out. A name the
	// repository already has fails.
	CreateBranch(dir string, branch string, start string) error
	// ResetBranch moves a branch to start and checks it out. A name the
	// repository does not have yet is created.
	ResetBranch(dir string, branch string, start string) error
	// DeleteBranch removes a branch, whatever it holds. A branch checked out
	// in a work tree fails.
	DeleteBranch(dir string, branch string) error
	// CurrentBranch returns the branch a work tree has checked out, and an
	// empty string for a detached HEAD.
	CurrentBranch(dir string) (string, error)
	// BranchesContaining returns the branches of a repository that reach a
	// commit.
	BranchesContaining(dir string, commit string) ([]string, error)
	// CheckoutDetached moves a work tree to ref with no branch on it.
	CheckoutDetached(dir string, ref string) error
	// ResolveCommit returns the commit a ref points at.
	ResolveCommit(dir string, ref string) (Commit, error)
	// ResolveHeadCommit returns the commit a work tree sits on.
	ResolveHeadCommit(dir string) (Commit, error)
}

// Commit is one commit of a repository.
type Commit struct {
	SHA         string
	CommittedAt time.Time
}

// shortSHALength is how much of a commit drudge prints.
const shortSHALength = 12

// ShortSHA cuts a commit down to what drudge prints.
func ShortSHA(commit string) string {
	if len(commit) <= shortSHALength {
		return commit
	}
	return commit[:shortSHALength]
}

// BranchHoldsNoWork reports whether a branch holds nothing base does not
// already have. A branch that is not there holds nothing.
func BranchHoldsNoWork(operations Operations, dir string, base string, branch string) (bool, error) {
	present, err := operations.BranchExists(dir, branch)
	if err != nil {
		return false, err
	}
	if !present {
		return true, nil
	}

	count, err := operations.CommitCount(dir, base, branch)
	if err != nil {
		return false, err
	}
	return count == 0, nil
}
