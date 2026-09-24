package drudger

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/IgorBolotnikov/DRUDGE/internal/common"
	"github.com/IgorBolotnikov/DRUDGE/internal/config"
	"github.com/IgorBolotnikov/DRUDGE/internal/git"
	"github.com/IgorBolotnikov/DRUDGE/internal/project"
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

// projectRepository is one repository of a project, whatever slot checks it
// out.
type projectRepository struct {
	// Name is the basename of the repository's path, which is the project
	// directory's name for a project that is itself a repository.
	Name string
	// Path is where the local config records the repository, relative to the
	// project directory.
	Path string
	// Dir is where the repository lives in the project directory.
	Dir string
	// GitDir is the repository's .git. A worktree's own .git is a file
	// pointing at it, so a sandbox without this mount has no working git.
	GitDir        string
	DefaultBranch string
	// HasRemote says whether the repository has an origin to fetch from.
	HasRemote bool
}

// BaseRef is what a repository cuts work from. A repository with a remote
// cuts from the tracking ref, which is never checked out anywhere and so can
// be moved by a fetch.
func (repository projectRepository) BaseRef() string {
	if repository.HasRemote {
		return git.OriginRemote + "/" + repository.DefaultBranch
	}
	return repository.DefaultBranch
}

// repositoryWorktree is one repository of a project as one Drudger sees it.
type repositoryWorktree struct {
	projectRepository
	// Worktree is this slot's checkout of the repository.
	Worktree string
}

// defaultBranch names what the repositories of a workspace cut work from.
// Repositories sharing one default branch render as that single name.
func (space slotWorkspace) defaultBranch() string {
	names := make([]string, 0, len(space.Repositories))
	for _, repository := range space.Repositories {
		if !slices.Contains(names, repository.DefaultBranch) {
			names = append(names, repository.DefaultBranch)
		}
	}
	return strings.Join(names, ", ")
}

// resolveWorkspace works out where a Drudger works and what each of its
// repositories cuts work from. It reads git and writes nothing, so a dry run
// can ask for it.
func (service *DrudgerService) resolveWorkspace(layout projectLayout, drudger *Drudger) (slotWorkspace, error) {
	repositories, err := service.resolveRepositories(layout)
	if err != nil {
		return slotWorkspace{}, err
	}

	space := slotWorkspace{Slot: drudger.Slot, Root: drudger.Workspace, Repositories: make([]repositoryWorktree, 0, len(repositories))}
	for _, repository := range repositories {
		space.Repositories = append(space.Repositories, repositoryWorktree{
			projectRepository: repository,
			Worktree:          filepath.Join(space.Root, repository.Path),
		})
	}
	return space, nil
}

// resolveRepositories works out where every repository of a project sits and
// what it cuts work from. It reads git and writes nothing. A project with no
// repositories recorded is refused.
func (service *DrudgerService) resolveRepositories(layout projectLayout) ([]projectRepository, error) {
	recorded := service.localCfg.Repositories
	if len(recorded) == 0 {
		return nil, fmt.Errorf(
			"project %s has no repositories recorded, run %s to record them",
			// TODO: make init rerunnable on the existing project, or create another command, like `sync`
			service.localCfg.ProjectSlug, initCommand,
		)
	}

	repositories := make([]projectRepository, 0, len(recorded))
	for _, repository := range recorded {
		resolved, err := service.resolveRepository(layout, repository)
		if err != nil {
			return nil, err
		}
		repositories = append(repositories, resolved)
	}
	return repositories, nil
}

// resolveRepository works out where one repository of a project sits and what
// it cuts work from.
func (service *DrudgerService) resolveRepository(layout projectLayout, repository config.Repository) (projectRepository, error) {
	branch, err := project.DefaultBranchOf(service.gitOps, layout.Dir, repository)
	if err != nil {
		return projectRepository{}, err
	}

	dir := filepath.Join(layout.Dir, repository.Path)
	hasRemote, err := service.gitOps.HasRemote(dir, git.OriginRemote)
	if err != nil {
		return projectRepository{}, err
	}

	return projectRepository{
		Name:          repositoryNameOf(dir),
		Path:          repository.Path,
		Dir:           dir,
		GitDir:        filepath.Join(dir, gitDirName),
		DefaultBranch: branch,
		HasRemote:     hasRemote,
	}, nil
}

// repositoryNameOf is the name a repository at a path is recorded under.
func repositoryNameOf(dir string) string {
	return filepath.Base(dir)
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

// fetchBase updates the tracking ref a repository cuts work from. A fetch that
// fails only warns and names the commit the work is cut from, because that
// base is still a correct one to branch from.
func (service *DrudgerService) fetchBase(repository repositoryWorktree) {
	if !service.tryFetchBase(repository.projectRepository) {
		service.logger.Error("Work on repository %s is cut from %s", repository.Name, service.describeBase(repository))
	}
}

// tryFetchBase updates the tracking ref a repository cuts work from and
// reports whether its base is fresh. A repository with no remote is left alone
// and its base counts as fresh. A fetch that fails is logged.
func (service *DrudgerService) tryFetchBase(repository projectRepository) bool {
	if !repository.HasRemote {
		return true
	}

	err := service.gitOps.Fetch(repository.Dir, git.OriginRemote, repository.DefaultBranch)
	if err == nil {
		return true
	}
	service.logger.Error("Could not fetch %s of repository %s: %v", repository.DefaultBranch, repository.Name, err)
	return false
}

// describeBase names the commit a repository cuts work from and how old it is.
// A ref git will not resolve is described by its name alone.
func (service *DrudgerService) describeBase(repository repositoryWorktree) string {
	base, err := service.gitOps.ResolveCommit(repository.Dir, repository.BaseRef())
	if err != nil {
		return repository.BaseRef()
	}
	return fmt.Sprintf("%s at %s, committed %s", repository.BaseRef(), git.ShortSHA(base.SHA), formatAge(time.Since(base.CommittedAt)))
}

// stashWorktree puts whatever a worktree holds uncommitted aside under a
// message and returns the commit holding it. A clean worktree is left alone
// and returns an empty commit.
func (service *DrudgerService) stashWorktree(repository repositoryWorktree, message string) (string, error) {
	isDirty, err := service.gitOps.IsDirty(repository.Worktree)
	if err != nil {
		return "", err
	}
	if !isDirty {
		return "", nil
	}

	commit, err := service.gitOps.Stash(repository.Worktree, message)
	if err != nil {
		return "", fmt.Errorf("could not stash what is uncommitted in the workspace of repository %s: %w", repository.Name, err)
	}

	service.logger.Info("Repository %s held uncommitted changes, they are stashed at %s", repository.Name, git.ShortSHA(commit))
	return commit, nil
}

// nukeWorktree takes the worktree of one repository out of a workspace and
// returns the commit holding whatever it had uncommitted. A worktree that is
// not on disk is only pruned, which clears the registration its repository
// still holds.
//
// A stash that fails leaves the worktree where it is, so work an agent never
// committed is not deleted unsaved.
func (service *DrudgerService) nukeWorktree(repository repositoryWorktree, message string) (string, error) {
	isPresent, err := common.Exists(repository.Worktree)
	if err != nil {
		return "", err
	}

	var stash string
	if isPresent {
		stash, err = service.stashWorktree(repository, message)
		if err != nil {
			return "", err
		}
		if err := service.gitOps.RemoveWorktree(repository.Dir, repository.Worktree); err != nil {
			return stash, err
		}
	}

	return stash, service.gitOps.PruneWorktrees(repository.Dir)
}

// ensureWorkspace makes a Drudger's workspace ready for a handover and records
// what it saw. Every repository has its base fetched and gets a worktree when
// the slot has none. A worktree no agent can be given stops the run and names
// the Drudger to nuke.
func (service *DrudgerService) ensureWorkspace(projectSlug string, space slotWorkspace) error {
	for _, repository := range space.Repositories {
		service.fetchBase(repository)

		health, err := service.ensureWorktree(repository)
		if err != nil {
			return err
		}
		if health != WorkspaceUsable {
			service.recordWorkspaceHealth(projectSlug, space.Slot, health)
			return refuseWorkspace(space.Slot, repository, health)
		}
	}

	service.recordWorkspaceHealth(projectSlug, space.Slot, WorkspaceUsable)
	return nil
}

// ensureWorktree reports whether the worktree of one repository is usable, and
// checks the repository out at a path that has none yet. A new worktree is
// detached at the base the repository cuts work from, so an idle Drudger owns
// no branch. A worktree that is already there is left as the last Session left
// it.
//
// What the repository knows tells the two failures apart. A path it has
// registered with no directory behind it is a worktree that was deleted, and a
// directory it has not registered is a path something else took.
func (service *DrudgerService) ensureWorktree(repository repositoryWorktree) (WorkspaceHealth, error) {
	isPresent, err := common.Exists(repository.Worktree)
	if err != nil {
		return "", err
	}
	isRegistered, err := service.gitOps.HasWorktree(repository.Dir, repository.Worktree)
	if err != nil {
		return "", err
	}

	switch {
	case isPresent && isRegistered:
		return WorkspaceUsable, nil
	case isPresent:
		return WorkspaceMisplaced, nil
	case isRegistered:
		return WorkspaceGone, nil
	}

	if err := service.gitOps.AddDetachedWorktree(repository.Dir, repository.Worktree, repository.BaseRef()); err != nil {
		return "", fmt.Errorf("could not create the workspace of repository %s: %w", repository.Name, err)
	}
	return WorkspaceUsable, nil
}

// refuseWorkspace explains a workspace no agent can be given and names the one
// command that fixes it. A sandbox binds its mounts when it is created. A
// worktree put back under a live sandbox is a directory the agent cannot see,
// so rebuilding the whole Drudger is the only repair.
func refuseWorkspace(slot int, repository repositoryWorktree, health WorkspaceHealth) error {
	saw := fmt.Sprintf("%s is not a worktree of repository %s at %s", repository.Worktree, repository.Name, repository.Dir)
	if health == WorkspaceGone {
		saw = fmt.Sprintf("the worktree of repository %s is gone from %s", repository.Name, repository.Worktree)
	}
	return fmt.Errorf("%s, so Drudger %d cannot work, run %s %d to rebuild it", saw, slot, nukeCommand, slot)
}

// formatAge renders roughly how long ago a commit was made.
func formatAge(elapsed time.Duration) string {
	switch {
	case elapsed < time.Hour:
		return fmt.Sprintf("%d minutes ago", int(elapsed.Minutes()))
	case elapsed < 24*time.Hour:
		return fmt.Sprintf("%d hours ago", int(elapsed.Hours()))
	default:
		return fmt.Sprintf("%d days ago", int(elapsed.Hours()/24))
	}
}
