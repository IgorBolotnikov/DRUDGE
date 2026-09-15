package drudger

import (
	"fmt"
	"path/filepath"

	"drudge/internal/common"
	"drudge/internal/config"
	"drudge/internal/git"
	"drudge/internal/project"
)

// gitDirName is the repository directory a sandbox mounts alongside a
// worktree.
const gitDirName = ".git"

// slotWorkspace is where the Drudger of one slot works. Root is the agent's
// working directory and holds the worktree of every repository the project
// has.
type slotWorkspace struct {
	Slot         int
	Root         string
	Repositories []repositoryWorktree
}

// repositoryWorktree is one repository of a project as one Drudger sees it.
type repositoryWorktree struct {
	// Name is the basename of the repository's path, which is the project
	// directory's name for a project that is itself a repository.
	Name string
	// Dir is where the repository lives in the project directory.
	Dir string
	// GitDir is the repository's .git. A worktree's own .git is a file
	// pointing at it, so a sandbox without this mount has no working git.
	GitDir string
	// Worktree is this slot's checkout of the repository.
	Worktree      string
	DefaultBranch string
	// Remote says whether the repository has an origin to fetch from.
	Remote bool
}

// BaseRef is what a repository cuts work from. A repository with a remote
// cuts from the tracking ref, which is never checked out anywhere and so can
// be moved by a fetch.
func (repository repositoryWorktree) BaseRef() string {
	if repository.Remote {
		return git.OriginRemote + "/" + repository.DefaultBranch
	}
	return repository.DefaultBranch
}

// resolveWorkspace works out where a Drudger works and what each of its
// repositories cuts work from. It reads git and writes nothing, so a dry run
// can ask for it. A project with no repositories recorded is refused.
func (service *DrudgerService) resolveWorkspace(layout projectLayout, drudger *Drudger) (slotWorkspace, error) {
	repositories := service.localCfg.Repositories
	if len(repositories) == 0 {
		return slotWorkspace{}, fmt.Errorf(
			"project %s has no repositories recorded, run %s to record them",
			// TODO: make init rerunnable on the existing project, or create another command, like `sync`
			service.localCfg.ProjectSlug, initCommand,
		)
	}

	space := slotWorkspace{Slot: drudger.Slot, Root: drudger.Workspace, Repositories: make([]repositoryWorktree, 0, len(repositories))}
	for _, repository := range repositories {
		resolved, err := service.resolveRepository(layout, space.Root, repository)
		if err != nil {
			return slotWorkspace{}, err
		}
		space.Repositories = append(space.Repositories, resolved)
	}
	return space, nil
}

// resolveRepository works out where one repository of a project sits and where
// this workspace checks it out.
func (service *DrudgerService) resolveRepository(layout projectLayout, root string, repository config.Repository) (repositoryWorktree, error) {
	branch, err := project.DefaultBranchOf(service.gitOps, layout.Dir, repository)
	if err != nil {
		return repositoryWorktree{}, err
	}

	dir := filepath.Join(layout.Dir, repository.Path)
	remote, err := service.gitOps.HasRemote(dir, git.OriginRemote)
	if err != nil {
		return repositoryWorktree{}, err
	}

	return repositoryWorktree{
		Name:          filepath.Base(dir),
		Dir:           dir,
		GitDir:        filepath.Join(dir, gitDirName),
		Worktree:      filepath.Join(root, repository.Path),
		DefaultBranch: branch,
		Remote:        remote,
	}, nil
}

// mounts lists the host paths a Drudger's sandbox is created over: the
// workspace root, the .git of every repository, and the directory the runs of
// the project live in.
func (space slotWorkspace) mounts(runsDir string) []string {
	// One mount for the root, one for the runs directory, one per repository.
	paths := make([]string, 0, len(space.Repositories)+2)
	paths = append(paths, space.Root)
	for _, repository := range space.Repositories {
		paths = append(paths, repository.GitDir)
	}
	return append(paths, runsDir)
}

// ensureWorktree checks a repository out in the worktree of a slot when it has
// none yet, detached at the base the repository cuts work from so an idle
// Drudger owns no branch. A worktree that is already there is left as the last
// Session left it.
func (service *DrudgerService) ensureWorktree(repository repositoryWorktree) error {
	present, err := common.Exists(repository.Worktree)
	if err != nil {
		return err
	}
	if present {
		return nil
	}

	if err := service.gitOps.AddDetachedWorktree(repository.Dir, repository.Worktree, repository.BaseRef()); err != nil {
		return fmt.Errorf("could not create the workspace of repository %s: %w", repository.Name, err)
	}
	return nil
}
