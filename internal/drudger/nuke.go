package drudger

import (
	"fmt"
	"slices"
	"time"

	"drudge/internal/common"
	"drudge/internal/task"
)

// NukeDrudger destroys a Drudger: its sandbox is deleted and its entry leaves
// the store. Nothing else removes a Drudger, so the slot is free for the next
// allocation to build a fresh one in.
//
// A Drudger whose Session is still running is refused. Forcing goes through
// anyway, which kills the agent along with the sandbox, so the task it was
// working on ends up fucked up. A Session that has already finished frees its
// Drudger first, the same way allocation does.
func (service *DrudgerService) NukeDrudger(projectSlug string, slot int, force bool) error {
	workspace, err := common.WorkDir()
	if err != nil {
		return fmt.Errorf("could not work out where the Drudgers of project %s run: %w", projectSlug, err)
	}

	var sandboxName string
	var killedTaskID task.TaskID

	err = service.drudgers.UpdateDrudgers(projectSlug, func(drudgers []*Drudger) ([]*Drudger, error) {
		if err := reclaimFinished(drudgers, workspace, time.Now().UTC()); err != nil {
			return nil, err
		}

		doomed := drudgerAtSlot(drudgers, slot)
		if doomed == nil {
			return nil, fmt.Errorf("project %s has no Drudger in slot %d, list the Drudgers to see the slots it has", projectSlug, slot)
		}
		if !doomed.Idle() && !force {
			return nil, fmt.Errorf("Drudger %d (%s) is working on task %s, wait for that Session to finish or force the removal to kill the agent along with the sandbox", doomed.Slot, doomed.Sandbox, doomed.TaskID)
		}

		remove, err := service.pickRemoveCommand(doomed.Sandbox)
		if err != nil {
			return nil, err
		}

		// The sandbox is removed under the lock, so nothing can claim this
		// Drudger between the guard above and the container going away. A
		// failed removal stores nothing, which keeps the entry and its record
		// of a container that is still there.
		if _, err := service.commands.Run(remove); err != nil {
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

// recordKilledTask marks the task whose agent died with its Drudger. Leaving it
// in progress would point at a Session that no longer exists.
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

// drudgerAtSlot returns the Drudger holding a slot, or nil when the slot has
// none.
func drudgerAtSlot(drudgers []*Drudger, slot int) *Drudger {
	for _, candidate := range drudgers {
		if candidate.Slot == slot {
			return candidate
		}
	}
	return nil
}
