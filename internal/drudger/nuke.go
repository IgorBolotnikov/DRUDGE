package drudger

import (
	"fmt"
	"slices"
	"time"

	"drudge/internal/common"
	"drudge/internal/task"
)

// NukeDrudger destroys a Drudger: deletes its sandbox and removes the entry
// from the store.
//
// Nuking is refused if Drudger Session is still running. Forcing goes through
// anyway, which kills the agent along with the sandbox and fucks up the task.
// If Session is finished, it works the same way allocation does.
func (service *DrudgerService) NukeDrudger(projectSlug string, slot int, force bool) error {
	workspace, err := common.WorkDir()
	if err != nil {
		return fmt.Errorf("could not work out where the Drudgers of project %s run: %w", projectSlug, err)
	}

	var sandboxName string
	var killedTaskID task.TaskID

	err = service.drudgers.UpdateDrudgers(projectSlug, func(drudgers []*Drudger) ([]*Drudger, error) {
		// Reclaim to get the up-to-date state of all Drudgers.
		if err := reclaimFinished(drudgers, workspace, time.Now().UTC()); err != nil {
			return nil, err
		}

		doomed := drudgerAtSlot(drudgers, slot)
		if doomed == nil {
			return nil, fmt.Errorf("project %s has no Drudger in slot %d, list the Drudgers to see the slots they have", projectSlug, slot)
		}
		if !doomed.Idle() && !force {
			return nil, fmt.Errorf("Drudger %d (%s) is working on task %s, wait for that Session to finish or force the removal to kill it regardless (beware that it will fuck up the task)", doomed.Slot, doomed.Sandbox, doomed.TaskID)
		}

		remove, err := service.pickRemoveCommand(doomed.Sandbox)
		if err != nil {
			return nil, err
		}

		// The sandbox is removed under the lock, so that nothing can claim this
		// Drudger in the meantime.
		if _, _, err := service.runSbx(remove); err != nil {
			return nil, fmt.Errorf("could not remove sandbox %s: %w", doomed.Sandbox, err)
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
	return service.recordKilledTask(projectSlug, killedTaskID)
}

// recordKilledTask marks the task whose agent died with its Drudger.
func (service *DrudgerService) recordKilledTask(projectSlug string, taskID task.TaskID) error {
	killed, err := service.tasks.GetTask(projectSlug, taskID)
	if err != nil {
		return fmt.Errorf("the Drudger is gone, but task %s could not be read to mark it %q: %w", taskID, task.StatusFuckedUp, err)
	}

	killed.Status = task.StatusFuckedUp
	killed.FinishedAt = time.Now().UTC()
	if err := service.tasks.UpdateTask(projectSlug, killed); err != nil {
		return fmt.Errorf("the Drudger is gone, but task %s could not be marked %q: %w", taskID, task.StatusFuckedUp, err)
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
