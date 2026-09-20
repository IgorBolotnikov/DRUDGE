package task

import (
	"errors"
	"fmt"
)

// errRemovalDeclined reports that the user answered no to the confirmation a
// removal asks for.
var errRemovalDeclined = errors.New("task removal declined")

// RunKeeper removes what the Sessions of a task left in the workspace.
type RunKeeper interface {
	// RemoveRun deletes the run directory of a task and reports whether the
	// task had one.
	RemoveRun(taskID TaskID) (bool, error)
}

// SessionKeeper answers for the Sessions of a task. The drudger service
// implements it, because the live Session check and the run directory are both
// read out of the workspace.
type SessionKeeper interface {
	SessionGuard
	RunKeeper
}

// ConfirmRemoval asks whether a task should go. It returns false to call the
// removal off.
type ConfirmRemoval func(taskToRemove *Task) (bool, error)

// RemoveTask deletes one task and the run directory of its Sessions. The id
// may be a prefix. A task whose agent is still working is refused and the
// error names the Drudger. isForced removes the task without asking. Any other
// removal goes ahead once confirm approves it.
func (service *TaskService) RemoveTask(projectSlug string, id TaskID, isForced bool, sessions SessionKeeper, confirm ConfirmRemoval) error {
	if id == "" {
		return ErrNoTaskID
	}

	found, err := service.repo.FindTask(projectSlug, string(id))
	if err != nil {
		return err
	}

	// Both checks run under the lock on the task, which catches a task an
	// agent picked up since the lookup.
	isRemoved, err := service.repo.DeleteTask(projectSlug, found.ID, func(taskToRemove *Task) error {
		if err := sessions.RefuseWhileWorking(projectSlug, taskToRemove); err != nil {
			return err
		}
		return approveRemoval(taskToRemove, isForced, confirm)
	})
	if errors.Is(err, errRemovalDeclined) {
		service.log.Info("Left task [%s] %s alone", found.ID, found.Title)
		return nil
	}
	if err != nil {
		return err
	}
	if !isRemoved {
		return fmt.Errorf("another drudge command is working on task %s, wait for it to finish and run this again", found.ID)
	}

	hasRun, err := sessions.RemoveRun(found.ID)
	if err != nil {
		return fmt.Errorf("task %s was removed, but its run directory was not: %w", found.ID, err)
	}

	service.log.Info("Removed task [%s] %s", found.ID, found.Title)
	if hasRun {
		service.log.Info("Its run directory went with it")
	}
	return nil
}

// approveRemoval puts the removal to the user and returns errRemovalDeclined
// when they turn it down. A forced removal asks nothing.
func approveRemoval(taskToRemove *Task, isForced bool, confirm ConfirmRemoval) error {
	if isForced {
		return nil
	}

	isApproved, err := confirm(taskToRemove)
	if err != nil {
		return fmt.Errorf("could not read the confirmation for task %s: %w", taskToRemove.ID, err)
	}
	if !isApproved {
		return errRemovalDeclined
	}
	return nil
}
