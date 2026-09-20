package drudger

import (
	"maps"
	"slices"

	"drudge/internal/common"
	"drudge/internal/git"
	"drudge/internal/task"
)

// RemoveRun deletes the run directory of a task and reports whether the task
// had one.
func (service *DrudgerService) RemoveRun(taskID task.TaskID) (bool, error) {
	layout, err := service.layout()
	if err != nil {
		return false, err
	}

	runDir := layout.RunDir(taskID)
	hasRunDir, err := common.Exists(runDir)
	if err != nil {
		return false, err
	}
	if !hasRunDir {
		return false, nil
	}

	if err := common.RemoveAll(runDir); err != nil {
		return false, err
	}
	return true, nil
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

	dirs := service.repositoryDirs(layout)
	for _, name := range slices.Sorted(maps.Keys(removed.Landings)) {
		landing := removed.Landings[name]

		dir, isRecorded := dirs[name]
		if !isRecorded {
			service.logger.Info("Branch %s stays, project %s records no repository %s", landing.Branch, service.localCfg.ProjectSlug, name)
			continue
		}
		service.removeEmptyBranch(name, dir, landing)
	}
	return nil
}

// removeEmptyBranch deletes the branch of one repository when it holds nothing
// its base does not already have. A branch holding commits is kept and named,
// and a branch that is already gone is passed over.
func (service *DrudgerService) removeEmptyBranch(name string, dir string, landing task.Landing) {
	hasBranch, err := service.gitOps.BranchExists(dir, landing.Branch)
	if err != nil {
		service.logger.Error("Could not read branch %s of repository %s: %v", landing.Branch, name, err)
		return
	}
	if !hasBranch {
		return
	}

	isEmpty, err := git.BranchHoldsNoWork(service.gitOps, dir, landing.Base, landing.Branch)
	if err != nil {
		service.logger.Error("Could not read what branch %s of repository %s holds: %v", landing.Branch, name, err)
		return
	}
	if !isEmpty {
		service.logger.Info("Branch %s of repository %s holds work, it stays", landing.Branch, name)
		return
	}

	if err := service.gitOps.DeleteBranch(dir, landing.Branch); err != nil {
		service.logger.Error("Could not delete branch %s of repository %s, it stays: %v", landing.Branch, name, err)
		return
	}
	service.logger.Info("Branch %s of repository %s held nothing, it is deleted", landing.Branch, name)
}
