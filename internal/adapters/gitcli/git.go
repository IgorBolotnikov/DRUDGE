// Package gitcli carries out git operations by running the git binary.
package gitcli

import (
	"errors"
	"fmt"
	osexec "os/exec"
	"path/filepath"
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
)

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
// compare equal.
func realPath(path string) (string, error) {
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", fmt.Errorf("could not resolve %s: %w", path, err)
	}
	return filepath.Clean(resolved), nil
}
