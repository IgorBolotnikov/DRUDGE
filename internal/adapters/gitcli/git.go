// Package gitcli carries out git operations by running the git binary.
package gitcli

import (
	"errors"
	"fmt"
	"os"
	osexec "os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"drudge/internal/adapters/exec"
	"drudge/internal/git"
)

// Pieces of a git invocation.
const (
	gitBinary             = "git"
	gitWorkDirFlag        = "-C"
	revParseSubcommand    = "rev-parse"
	showTopLevelFlag      = "--show-toplevel"
	symbolicRefSubcommand = "symbolic-ref"
	shortFlag             = "--short"
	originHeadRef         = "refs/remotes/origin/HEAD"
	originPrefix          = "origin/"
	remoteSubcommand      = "remote"
	getURLSubcommand      = "get-url"
	fetchSubcommand       = "fetch"
	worktreeSubcommand    = "worktree"
	addSubcommand         = "add"
	removeSubcommand      = "remove"
	pruneSubcommand       = "prune"
	forceFlag             = "--force"
	listSubcommand        = "list"
	detachFlag            = "--detach"
	statusSubcommand      = "status"
	porcelainFlag         = "--porcelain"
	stashSubcommand       = "stash"
	pushSubcommand        = "push"
	includeUntrackedFlag  = "--include-untracked"
	messageFlag           = "-m"
	stashRef              = "refs/stash"
	branchRefPrefix       = "refs/heads/"
	verifyFlag            = "--verify"
	quietFlag             = "-q"
	revListSubcommand     = "rev-list"
	countFlag             = "--count"
	switchSubcommand      = "switch"
	createBranchFlag      = "-c"
	resetBranchFlag       = "-C"
	branchSubcommand      = "branch"
	deleteBranchFlag      = "-D"
	forEachRefSubcommand  = "for-each-ref"
	containsFlag          = "--contains"
	headRef               = "HEAD"
	// refNameFormatFlag prints one ref name per line, with none of the
	// decoration git adds for a terminal.
	refNameFormatFlag = "--format=%(refname:short)"
	showSubcommand    = "show"
	noPatchFlag       = "--no-patch"
	// commitFormatFlag prints a commit as its sha and its committer date.
	commitFormatFlag = "--format=%H%n%cI"
)

// commitRange is the two-dot range git reads "commits in tip that base does
// not have" from.
const commitRange = "%s..%s"

// worktreeField starts the porcelain listing line that carries a worktree's
// path.
const worktreeField = "worktree "

// Git runs git commands as processes.
type Git struct {
	runner   *exec.CommandRunner
	timeouts git.Timeouts
}

func New(runner *exec.CommandRunner, timeouts git.Timeouts) *Git {
	return &Git{runner: runner, timeouts: timeouts}
}

// IsRepositoryRoot reports whether dir is the root of a git work tree. A
// directory git refuses reports false, which covers a missing directory, a
// plain directory and a bare repository. A subdirectory of a repository
// reports false too, because git answers with the root above it.
func (adapter *Git) IsRepositoryRoot(dir string) (bool, error) {
	stdout, _, err := adapter.run(dir, adapter.timeouts.Command, revParseSubcommand, showTopLevelFlag)
	if err != nil {
		if refused(err) {
			return false, nil
		}
		return false, err
	}

	top, err := realPath(strings.TrimSpace(stdout))
	if err != nil {
		return false, err
	}
	asked, err := realPath(dir)
	if err != nil {
		return false, err
	}
	return top == asked, nil
}

// DefaultBranch returns the branch origin/HEAD points at, without the remote
// prefix. Any refusal from git returns git.ErrNoDefaultBranch, so a caller
// confirms the directory is a repository before it asks.
func (adapter *Git) DefaultBranch(dir string) (string, error) {
	stdout, _, err := adapter.run(dir, adapter.timeouts.Command, symbolicRefSubcommand, shortFlag, originHeadRef)
	if err != nil {
		if refused(err) {
			return "", git.ErrNoDefaultBranch
		}
		return "", err
	}
	return strings.TrimPrefix(strings.TrimSpace(stdout), originPrefix), nil
}

// HasRemote reports whether a repository has the named remote. A directory
// that is not a repository reports false.
func (adapter *Git) HasRemote(dir string, remote string) (bool, error) {
	_, _, err := adapter.run(dir, adapter.timeouts.Command, remoteSubcommand, getURLSubcommand, remote)
	if err != nil {
		if refused(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// Fetch updates the tracking ref of one branch of a remote. A remote git
// cannot reach fails.
func (adapter *Git) Fetch(dir string, remote string, branch string) error {
	_, stderr, err := adapter.run(dir, adapter.timeouts.Fetch, fetchSubcommand, remote, branch)
	if err != nil {
		return fmt.Errorf("could not fetch %s %s in %s: %w: %s", remote, branch, dir, err, strings.TrimSpace(stderr))
	}
	return nil
}

// AddDetachedWorktree checks a repository out at ref in a new worktree at
// path, with no branch on it. Git creates the path, takes over an empty
// directory, and refuses one holding files.
func (adapter *Git) AddDetachedWorktree(dir string, path string, ref string) error {
	_, stderr, err := adapter.run(dir, adapter.timeouts.Worktree, worktreeSubcommand, addSubcommand, detachFlag, path, ref)
	if err != nil {
		return fmt.Errorf("could not create a worktree of %s at %s on %s: %w: %s", dir, path, ref, err, strings.TrimSpace(stderr))
	}
	return nil
}

// RemoveWorktree deletes the directory of a worktree and the registration the
// repository holds for it.
func (adapter *Git) RemoveWorktree(dir string, path string) error {
	// Forcing covers a worktree holding uncommitted changes, which git refuses
	// to remove otherwise.
	_, stderr, err := adapter.run(dir, adapter.timeouts.Worktree, worktreeSubcommand, removeSubcommand, forceFlag, path)
	if err != nil {
		return fmt.Errorf("could not remove the worktree of %s at %s: %w: %s", dir, path, err, strings.TrimSpace(stderr))
	}
	return nil
}

// PruneWorktrees drops the registrations of worktrees whose directory is gone.
func (adapter *Git) PruneWorktrees(dir string) error {
	_, stderr, err := adapter.run(dir, adapter.timeouts.Worktree, worktreeSubcommand, pruneSubcommand)
	if err != nil {
		return fmt.Errorf("could not prune the worktrees of %s: %w: %s", dir, err, strings.TrimSpace(stderr))
	}
	return nil
}

// HasWorktree reports whether path is registered as a worktree of the
// repository at dir. The listing carries every worktree the repository knows,
// including the main work tree and worktrees whose directory was deleted.
func (adapter *Git) HasWorktree(dir string, path string) (bool, error) {
	stdout, stderr, err := adapter.run(dir, adapter.timeouts.Command, worktreeSubcommand, listSubcommand, porcelainFlag)
	if err != nil {
		return false, fmt.Errorf("could not list the worktrees of %s: %w: %s", dir, err, strings.TrimSpace(stderr))
	}

	wanted, err := realPath(path)
	if err != nil {
		return false, err
	}

	for _, line := range strings.Split(stdout, "\n") {
		listed, hasPrefix := strings.CutPrefix(strings.TrimSpace(line), worktreeField)
		if !hasPrefix {
			continue
		}
		resolved, err := realPath(listed)
		if err != nil {
			return false, err
		}
		if resolved == wanted {
			return true, nil
		}
	}
	return false, nil
}

// IsDirty reports whether a work tree holds changes that are not committed.
// Untracked files count and ignored files do not, which is what git reports by
// default.
func (adapter *Git) IsDirty(dir string) (bool, error) {
	stdout, _, err := adapter.run(dir, adapter.timeouts.Command, statusSubcommand, porcelainFlag)
	if err != nil {
		return false, fmt.Errorf("could not read the status of %s: %w", dir, err)
	}
	return strings.TrimSpace(stdout) != "", nil
}

// Stash puts the uncommitted changes of a work tree aside under a message and
// returns the commit holding them. The stash the repository already had is
// read first, so a work tree git found nothing to stash returns an empty
// commit rather than the commit of somebody else's stash.
func (adapter *Git) Stash(dir string, message string) (string, error) {
	before, err := adapter.revision(dir, stashRef)
	if err != nil {
		return "", err
	}

	_, stderr, err := adapter.run(dir, adapter.timeouts.Command, stashSubcommand, pushSubcommand, includeUntrackedFlag, messageFlag, message)
	if err != nil {
		return "", fmt.Errorf("could not stash the changes in %s: %w: %s", dir, err, strings.TrimSpace(stderr))
	}

	after, err := adapter.revision(dir, stashRef)
	if err != nil {
		return "", err
	}
	if after == before {
		return "", nil
	}
	return after, nil
}

// BranchExists reports whether a repository has a branch of this name.
func (adapter *Git) BranchExists(dir string, branch string) (bool, error) {
	commit, err := adapter.revision(dir, branchRefPrefix+branch)
	if err != nil {
		return false, err
	}
	return commit != "", nil
}

// CommitCount returns how many commits tip holds that base does not. A ref git
// cannot resolve fails.
func (adapter *Git) CommitCount(dir string, base string, tip string) (int, error) {
	stdout, stderr, err := adapter.run(dir, adapter.timeouts.Command, revListSubcommand, countFlag, fmt.Sprintf(commitRange, base, tip))
	if err != nil {
		return 0, fmt.Errorf("could not count the commits of %s beyond %s in %s: %w: %s", tip, base, dir, err, strings.TrimSpace(stderr))
	}

	count, err := strconv.Atoi(strings.TrimSpace(stdout))
	if err != nil {
		return 0, fmt.Errorf("git answered %q when asked how many commits %s holds beyond %s in %s", strings.TrimSpace(stdout), tip, base, dir)
	}
	return count, nil
}

// CreateBranch creates a branch at start and checks it out. A name the
// repository already has fails.
func (adapter *Git) CreateBranch(dir string, branch string, start string) error {
	return adapter.switchTo(dir, createBranchFlag, branch, start)
}

// ResetBranch moves a branch to start and checks it out. A name the repository
// does not have yet is created.
func (adapter *Git) ResetBranch(dir string, branch string, start string) error {
	return adapter.switchTo(dir, resetBranchFlag, branch, start)
}

// switchTo checks a branch out at start, creating it the way flag says.
func (adapter *Git) switchTo(dir string, flag string, branch string, start string) error {
	_, stderr, err := adapter.run(dir, adapter.timeouts.Command, switchSubcommand, flag, branch, start)
	if err != nil {
		return fmt.Errorf("could not check out branch %s at %s in %s: %w: %s", branch, start, dir, err, strings.TrimSpace(stderr))
	}
	return nil
}

// DeleteBranch removes a branch, whatever it holds. A branch checked out in a
// work tree of the repository fails.
func (adapter *Git) DeleteBranch(dir string, branch string) error {
	_, stderr, err := adapter.run(dir, adapter.timeouts.Command, branchSubcommand, deleteBranchFlag, branch)
	if err != nil {
		return fmt.Errorf("could not delete branch %s in %s: %w: %s", branch, dir, err, strings.TrimSpace(stderr))
	}
	return nil
}

// CurrentBranch returns the branch a work tree has checked out. Git refuses on
// a detached HEAD, which reads back as an empty string.
func (adapter *Git) CurrentBranch(dir string) (string, error) {
	stdout, _, err := adapter.run(dir, adapter.timeouts.Command, symbolicRefSubcommand, shortFlag, quietFlag, headRef)
	if err != nil {
		if refused(err) {
			return "", nil
		}
		return "", fmt.Errorf("could not read which branch %s has checked out: %w", dir, err)
	}
	return strings.TrimSpace(stdout), nil
}

// BranchesContaining returns the branches of a repository that reach a commit.
// It reads refs/heads directly, because the branch listing carries a row for a
// detached HEAD too.
func (adapter *Git) BranchesContaining(dir string, commit string) ([]string, error) {
	stdout, stderr, err := adapter.run(dir, adapter.timeouts.Command, forEachRefSubcommand, refNameFormatFlag, containsFlag, commit, branchRefPrefix)
	if err != nil {
		return nil, fmt.Errorf("could not list the branches of %s reaching %s: %w: %s", dir, commit, err, strings.TrimSpace(stderr))
	}

	var branches []string
	for _, line := range strings.Split(stdout, "\n") {
		if name := strings.TrimSpace(line); name != "" {
			branches = append(branches, name)
		}
	}
	return branches, nil
}

// CheckoutDetached moves a work tree to ref with no branch on it.
func (adapter *Git) CheckoutDetached(dir string, ref string) error {
	_, stderr, err := adapter.run(dir, adapter.timeouts.Command, switchSubcommand, detachFlag, ref)
	if err != nil {
		return fmt.Errorf("could not detach %s at %s: %w: %s", dir, ref, err, strings.TrimSpace(stderr))
	}
	return nil
}

// ResolveCommit returns the commit a ref points at. A ref git cannot resolve
// fails.
func (adapter *Git) ResolveCommit(dir string, ref string) (git.Commit, error) {
	stdout, stderr, err := adapter.run(dir, adapter.timeouts.Command, showSubcommand, noPatchFlag, commitFormatFlag, ref)
	if err != nil {
		return git.Commit{}, fmt.Errorf("could not resolve %s in %s: %w: %s", ref, dir, err, strings.TrimSpace(stderr))
	}

	sha, committedAt, hasCommittedAt := strings.Cut(strings.TrimSpace(stdout), "\n")
	if !hasCommittedAt {
		return git.Commit{}, fmt.Errorf("git described %s in %s as %q, which is not a commit and a date", ref, dir, strings.TrimSpace(stdout))
	}

	when, err := time.Parse(time.RFC3339, strings.TrimSpace(committedAt))
	if err != nil {
		return git.Commit{}, fmt.Errorf("could not read when %s of %s was committed: %w", ref, dir, err)
	}
	return git.Commit{SHA: sha, CommittedAt: when}, nil
}

// ResolveHeadCommit returns the commit a work tree sits on. A repository with
// no commits is an error.
func (adapter *Git) ResolveHeadCommit(dir string) (git.Commit, error) {
	return adapter.ResolveCommit(dir, headRef)
}

// revision returns the commit a ref points at, and an empty string when the
// repository has no such ref.
func (adapter *Git) revision(dir string, ref string) (string, error) {
	stdout, _, err := adapter.run(dir, adapter.timeouts.Command, revParseSubcommand, verifyFlag, quietFlag, ref)
	if err != nil {
		if refused(err) {
			return "", nil
		}
		return "", fmt.Errorf("could not resolve %s in %s: %w", ref, dir, err)
	}
	return strings.TrimSpace(stdout), nil
}

// run executes a git command in dir.
func (adapter *Git) run(dir string, timeout time.Duration, args ...string) (stdout string, stderr string, err error) {
	argv := append([]string{gitBinary, gitWorkDirFlag, dir}, args...)
	return adapter.runner.Run(argv, timeout)
}

// refused tells whether git ran and exited non-zero. Any other failure means
// git did not run: a missing binary, or a command killed on its timeout.
func refused(err error) bool {
	var exitErr *osexec.ExitError
	return errors.As(err, &exitErr)
}

// realPath resolves a path through symlinks so two names for one directory
// compare equal. A path that is not there is resolved through the deepest
// parent that is. A worktree the repository still knows can have no directory
// left.
func realPath(path string) (string, error) {
	prefix := filepath.Clean(path)
	suffix := ""

	for {
		resolved, err := filepath.EvalSymlinks(prefix)
		if err == nil {
			return filepath.Join(resolved, suffix), nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("could not resolve %s: %w", path, err)
		}

		parent := filepath.Dir(prefix)
		if parent == prefix {
			return filepath.Join(prefix, suffix), nil
		}
		// The component goes in front of the suffix, so the two halves still
		// join back into the path that was asked about.
		suffix = filepath.Join(filepath.Base(prefix), suffix)
		prefix = parent
	}
}
