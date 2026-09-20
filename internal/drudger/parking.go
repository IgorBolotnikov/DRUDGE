package drudger

import (
	"fmt"

	"drudge/internal/common"
	"drudge/internal/task"
)

// parkWorkspace parks every repository of a workspace and records the stashes
// it makes on parked, which is nil when no task is known to have left the
// changes. A worktree that is not on disk is skipped.
//
// Parking frees the branch a run was on, because git refuses to delete a
// branch a worktree holds. A workspace that is already parked is only read, so
// any command can park whatever it finds.
func (service *DrudgerService) parkWorkspace(space slotWorkspace, parked *task.Task) error {
	for _, repository := range space.Repositories {
		isPresent, err := common.Exists(repository.Worktree)
		if err != nil {
			return err
		}
		if !isPresent {
			continue
		}

		if err := service.parkRepository(space.Slot, repository, parked); err != nil {
			return err
		}
	}
	return nil
}

// parkRepository stashes what one worktree holds uncommitted and detaches it
// at the base its repository cuts work from. A worktree that is already
// detached is left where it sits.
func (service *DrudgerService) parkRepository(slot int, repository repositoryWorktree, parked *task.Task) error {
	stash, err := service.stashWorktree(repository, parkingStashMessage(slot, parked))
	if err != nil {
		return err
	}
	if parked != nil {
		parked.RecordStash(repository.Name, stash)
	}

	branch, err := service.gitOps.CurrentBranch(repository.Worktree)
	if err != nil {
		return err
	}
	if branch == "" {
		return nil
	}

	if err := service.gitOps.CheckoutDetached(repository.Worktree, repository.BaseRef()); err != nil {
		return fmt.Errorf("could not take the workspace of repository %s off branch %s: %w", repository.Name, branch, err)
	}
	return nil
}

// parkIdleDrudgers parks every Drudger of a pool that holds no task. A Drudger
// that could not be parked is reported and the sweep goes on to the next one.
func (service *DrudgerService) parkIdleDrudgers(projectSlug string, layout projectLayout, drudgers []*Drudger) {
	for _, candidate := range drudgers {
		if !candidate.Idle() {
			continue
		}
		// The worktree paths worked out from an empty root land in the
		// project's own checkout, which parking would stash and detach.
		if candidate.Workspace == "" {
			continue
		}

		space, err := service.resolveWorkspace(layout, candidate)
		if err != nil {
			service.logger.Error("Drudger %d of project %s holds no task, but the workspace it works in could not be read: %v", candidate.Slot, projectSlug, err)
			continue
		}
		if err := service.parkWorkspace(space, nil); err != nil {
			service.logger.Error("Drudger %d of project %s holds no task, but its workspace could not be parked: %v", candidate.Slot, projectSlug, err)
		}
	}
}

// parkingStashMessage names the slot a parking stash came from, and the task
// that left the changes when there is one to name.
func parkingStashMessage(slot int, parked *task.Task) string {
	if parked == nil {
		return fmt.Sprintf("drudge: slot %d parked", slot)
	}
	return fmt.Sprintf("drudge: slot %d after task %s %s", slot, task.ShortID(parked.ID), parked.Title)
}
