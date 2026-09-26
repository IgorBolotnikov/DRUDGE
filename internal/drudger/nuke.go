package drudger

import (
	"fmt"
	"slices"
	"time"

	"github.com/IgorBolotnikov/DRUDGE/internal/task"
)

// NukeDrudger destroys a Drudger: takes its workspace apart, deletes its
// sandbox and removes the entry from the store.
//
// Nuking is refused if Drudger Session is still running. Forcing goes through
// anyway, which kills the agent along with the sandbox and fucks up the task.
// If Session is finished, it works the same way allocation does.
//
// The branches the Drudger's tasks made are left alone. A workspace that
// cannot be taken apart is reported and the Drudger goes regardless.
func (service *DrudgerService) NukeDrudger(projectSlug string, slot int, isForced bool) error {
	layout, err := service.layout()
	if err != nil {
		return err
	}

	var sandboxName string
	var killedTaskID task.TaskID
	var stashes map[string]string

	err = service.drudgers.UpdateDrudgers(projectSlug, func(drudgers []*Drudger) ([]*Drudger, error) {
		// Reclaim to get the up-to-date state of all Drudgers.
		if err := reclaimFinished(drudgers, layout, time.Now().UTC()); err != nil {
			return nil, err
		}

		doomed := drudgerAtSlot(drudgers, slot)
		if doomed == nil {
			return nil, fmt.Errorf("project %s has no Drudger in slot %d, list the Drudgers to see the slots they have", projectSlug, slot)
		}
		if !doomed.Idle() && !isForced {
			return nil, fmt.Errorf("Drudger %d (%s) is working on task %s, wait for that Session to finish or force the removal to kill it regardless (beware that it will fuck up the task)", doomed.Slot, doomed.Sandbox, doomed.TaskID)
		}

		remove, err := service.pickRemoveCommand(doomed.Sandbox)
		if err != nil {
			return nil, err
		}

		// The workspace is taken apart under the lock too, so nothing can
		// hand a task to this Drudger while its worktrees are going.
		stashes = service.nukeWorkspace(projectSlug, layout, doomed)

		// The sandbox is removed under the lock, so that nothing can claim this
		// Drudger in the meantime.
		if err := service.removeSandbox(remove, doomed.Sandbox); err != nil {
			return nil, err
		}

		sandboxName = doomed.Sandbox
		killedTaskID = doomed.TaskID
		return slices.DeleteFunc(drudgers, func(candidate *Drudger) bool {
			return candidate.Slot == slot
		}), nil
	})
	if err != nil {
		return err
	}

	service.logger.Info("Drudger %d is gone, sandbox %s was deleted", slot, sandboxName)

	if killedTaskID == "" {
		return nil
	}
	return service.recordKilledTask(projectSlug, killedTaskID, stashes)
}

// removeSandbox deletes a Drudger's sandbox. A removal that fails on a sandbox
// the listing no longer holds counts as done.
func (service *DrudgerService) removeSandbox(remove sandboxCommand, sandboxName string) error {
	_, _, removeErr := service.runSbx(remove)
	if removeErr == nil {
		return nil
	}

	isGone, err := service.isSandboxGone(sandboxName)
	if err != nil {
		return fmt.Errorf("could not remove sandbox %s: %w, and could not check whether it is still there: %w", sandboxName, removeErr, err)
	}
	if !isGone {
		return fmt.Errorf("could not remove sandbox %s: %w", sandboxName, removeErr)
	}

	service.logger.Info("Sandbox %s was already gone", sandboxName)
	return nil
}

func (service *DrudgerService) isSandboxGone(sandboxName string) (bool, error) {
	inspect, err := service.pickInspectCommand()
	if err != nil {
		return false, err
	}
	listing, err := service.listSandboxes(inspect)
	if err != nil {
		return false, err
	}
	existing, err := findSandbox(listing, sandboxName)
	if err != nil {
		return false, err
	}
	return existing == nil, nil
}

// nukeWorkspace takes a Drudger's workspace apart and returns the commit the
// uncommitted changes of each repository were stashed at, keyed by repository
// name. A repository it cannot take apart is reported and the rest are still
// taken apart.
func (service *DrudgerService) nukeWorkspace(projectSlug string, layout projectLayout, doomed *Drudger) map[string]string {
	// The worktree paths worked out from an empty root land in the project's
	// own checkout, which a nuke would delete.
	if doomed.Workspace == "" {
		return nil
	}

	space, err := service.resolveWorkspace(layout, doomed)
	if err != nil {
		service.logger.Error("Drudger %d of project %s is being nuked, but the workspace it works in could not be read: %v", doomed.Slot, projectSlug, err)
		return nil
	}

	stashes := map[string]string{}
	for _, repository := range space.Repositories {
		stash, err := service.nukeWorktree(repository, nukeStashMessage(space.Slot, doomed.TaskID))
		if stash != "" {
			stashes[repository.Name] = stash
		}
		if err != nil {
			service.logger.Error("Drudger %d of project %s is being nuked, but its worktree of repository %s could not be taken out: %v", doomed.Slot, projectSlug, repository.Name, err)
		}
	}
	return stashes
}

// nukeStashMessage names the slot a nuke stashed a worktree from, and the task
// its agent was working on when there is one.
func nukeStashMessage(slot int, taskID task.TaskID) string {
	if taskID == "" {
		return fmt.Sprintf("drudge: slot %d nuked", slot)
	}
	return fmt.Sprintf("drudge: slot %d nuked during task %s", slot, task.ShortID(taskID))
}

// recordKilledTask marks the task whose agent died with its Drudger and keeps
// the stashes the nuke made in the workspace it ran in.
func (service *DrudgerService) recordKilledTask(projectSlug string, taskID task.TaskID, stashes map[string]string) error {
	var killed *task.Task
	isRecorded := false

	err := service.tasks.UpdateTask(projectSlug, taskID, func(stored *task.Task) error {
		killed = stored
		// A task the slot was read against may have moved on since. Only a task
		// still recorded as running was what the dead agent was working on.
		hasMovedOn := stored.Status != task.StatusInProgress
		if hasMovedOn && len(stashes) == 0 {
			return task.ErrTaskUnchanged
		}

		for repository, commit := range stashes {
			stored.RecordStash(repository, commit)
		}
		if hasMovedOn {
			return nil
		}

		stored.Status = task.StatusFuckedUp
		stored.FinishedAt = time.Now().UTC()
		isRecorded = true
		return nil
	})
	if err != nil {
		return fmt.Errorf("the Drudger is gone, but task %s could not be marked %q: %w", taskID, task.StatusFuckedUp, err)
	}
	if !isRecorded {
		return nil
	}

	service.logger.Info("Task [%s] %s is %s, its agent was killed with the Drudger", killed.ID, killed.Title, task.StatusFuckedUp)
	return nil
}

// drudgerAtSlot returns the Drudger holding a slot, or nil if none.
func drudgerAtSlot(drudgers []*Drudger, slot int) *Drudger {
	for _, candidate := range drudgers {
		if candidate.Slot == slot {
			return candidate
		}
	}
	return nil
}
