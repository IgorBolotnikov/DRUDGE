package drudger

import (
	"maps"
	"path/filepath"
	"slices"

	"github.com/IgorBolotnikov/DRUDGE/internal/git"
	"github.com/IgorBolotnikov/DRUDGE/internal/project"
	"github.com/IgorBolotnikov/DRUDGE/internal/task"
)

// RemoveRun deletes the run directory of a task and reports whether the task
// had one.
func (service *DrudgerService) RemoveRun(taskID task.TaskID) (bool, error) {
	return service.runs.RemoveRun(taskID)
}

// BranchOfUnknownRepositoryKept reports a branch RemoveEmptyBranches keeps
// because the project records no repository of that name.
type BranchOfUnknownRepositoryKept struct {
	ProjectSlug string
	Repository  string
	Branch      string
}

// BranchWithCommitsKept reports a branch RemoveEmptyBranches keeps because it
// holds commits.
type BranchWithCommitsKept struct {
	Repository string
	Branch     string
}

// EmptyBranchRemoved reports a branch RemoveEmptyBranches deleted because it
// held nothing.
type EmptyBranchRemoved struct {
	Repository string
	Branch     string
}

// BranchCleanupStep names the step of cleaning up a branch that failed.
type BranchCleanupStep string

const (
	RepositoryReadStep BranchCleanupStep = "read repository"
	BranchReadStep     BranchCleanupStep = "read branch"
	BranchInspectStep  BranchCleanupStep = "inspect branch"
	BranchDeleteStep   BranchCleanupStep = "delete branch"
)

// BranchCleanupFailed reports a branch RemoveEmptyBranches could not clean up.
type BranchCleanupFailed struct {
	Repository string
	Branch     string
	Step       BranchCleanupStep
	Err        error
}

// RemoveEmptyBranches deletes the branch a task left in every repository where
// it holds no commits, and names the branches it keeps. A task that never ran
// records no branch and reads no git.
//
// A repository whose branch cannot be read or deleted is reported, and the
// other repositories are still cleaned up.
func (service *DrudgerService) RemoveEmptyBranches(removed *task.Task) error {
	if len(removed.Landings) == 0 {
		return nil
	}

	layout, err := service.layout()
	if err != nil {
		return err
	}

	for _, name := range slices.Sorted(maps.Keys(removed.Landings)) {
		landing := removed.Landings[name]

		recorded, isRecorded := service.recordedRepository(layout, name)
		if !isRecorded {
			service.progress.Report(BranchOfUnknownRepositoryKept{ProjectSlug: service.settings.ProjectSlug, Repository: name, Branch: landing.Branch})
			continue
		}
		repository, err := service.resolveRepository(layout, recorded)
		if err != nil {
			service.progress.Report(BranchCleanupFailed{Repository: name, Branch: landing.Branch, Step: RepositoryReadStep, Err: err})
			continue
		}
		service.removeEmptyBranch(name, repository.Dir, landing)
	}
	return nil
}

// recordedRepository finds the repository of the local config that a name
// stands for.
func (service *DrudgerService) recordedRepository(layout projectLayout, name string) (project.Repository, bool) {
	for _, repository := range service.settings.Repositories {
		if repositoryNameOf(filepath.Join(layout.Dir, repository.Path)) == name {
			return repository, true
		}
	}
	return project.Repository{}, false
}

// removeEmptyBranch deletes the branch of one repository when it holds nothing
// its base does not already have. A branch holding commits is kept and named,
// and a branch that is already gone is passed over.
func (service *DrudgerService) removeEmptyBranch(name string, dir string, landing task.Landing) {
	hasBranch, err := service.gitOps.BranchExists(dir, landing.Branch)
	if err != nil {
		service.progress.Report(BranchCleanupFailed{Repository: name, Branch: landing.Branch, Step: BranchReadStep, Err: err})
		return
	}
	if !hasBranch {
		return
	}

	hasCommits, err := git.BranchHasCommits(service.gitOps, dir, landing.Base, landing.Branch)
	if err != nil {
		service.progress.Report(BranchCleanupFailed{Repository: name, Branch: landing.Branch, Step: BranchInspectStep, Err: err})
		return
	}
	if hasCommits {
		service.progress.Report(BranchWithCommitsKept{Repository: name, Branch: landing.Branch})
		return
	}

	if err := service.gitOps.DeleteBranch(dir, landing.Branch); err != nil {
		service.progress.Report(BranchCleanupFailed{Repository: name, Branch: landing.Branch, Step: BranchDeleteStep, Err: err})
		return
	}
	service.progress.Report(EmptyBranchRemoved{Repository: name, Branch: landing.Branch})
}
