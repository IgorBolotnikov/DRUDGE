package task

import (
	"errors"
	"fmt"
	"slices"
)

// errRemovalDeclined reports that the user answered no to the confirmation a
// removal asks for.
var errRemovalDeclined = errors.New("task removal declined")

// RunKeeper removes what the Sessions of a task left in the workspace.
type RunKeeper interface {
	// RemoveRun deletes the run directory of a task and reports whether the
	// task had one.
	RemoveRun(taskID TaskID) (bool, error)
	// RemoveEmptyBranches deletes the branches of a task that hold no commits
	// beyond the base they were cut from, and reports the ones it keeps.
	RemoveEmptyBranches(removed *Task) error
}

// SessionKeeper answers for the Sessions of a task. The drudger service
// implements it, because the live Session check and the run directory are both
// read out of the workspace.
type SessionKeeper interface {
	SessionGuard
	RunKeeper
}

// Removal is a task about to be removed and the tasks that name it. Both lists
// are oldest first, and a task can be on both.
type Removal struct {
	Task       *Task
	Dependents []*Task // Tasks blocked by Task
	Children   []*Task // Tasks belonging to Task
}

// ConfirmRemoval asks whether a task should go. It returns false to call the
// removal off.
type ConfirmRemoval func(removal Removal) (bool, error)

// RemoveTask deletes one task and the run directory of its Sessions, then
// takes the task off the blockers of its dependents and ungroups its children.
// The id may be a prefix. A task whose agent is still working is refused and
// the error names the Drudger. isForced removes the task without asking. Any
// other removal goes ahead once confirm approves it.
func (service *TaskService) RemoveTask(projectSlug string, id TaskID, isForced bool, sessions SessionKeeper, confirm ConfirmRemoval) error {
	if id == "" {
		return ErrNoTaskID
	}

	found, err := service.repo.FindTask(projectSlug, string(id))
	if err != nil {
		return err
	}

	tasks, err := service.repo.ListTasks(projectSlug)
	if err != nil {
		return fmt.Errorf("could not read the tasks that name task %s: %w", found.ID, err)
	}
	dependents, children := linkedTasks(tasks, found.ID)

	// Both checks run under the lock on the task, which catches a task an
	// agent picked up since the lookup.
	isRemoved, err := service.repo.DeleteTask(projectSlug, found.ID, func(taskToRemove *Task) error {
		if err := sessions.RefuseWhileWorking(projectSlug, taskToRemove); err != nil {
			return err
		}
		removal := Removal{Task: taskToRemove, Dependents: dependents, Children: children}
		return approveRemoval(removal, isForced, confirm)
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

	// The task file is already gone. A cleanup that fails is reported and the
	// removal stands.
	if err := sessions.RemoveEmptyBranches(found); err != nil {
		service.log.Error("Task %s is removed, but the branches it left could not be cleaned up: %v", found.ID, err)
	}
	service.unlink(projectSlug, found.ID, dependents, children)
	return nil
}

// linkedTasks returns the tasks blocked by id and the tasks belonging to it,
// oldest first.
func linkedTasks(tasks []*Task, id TaskID) (dependents []*Task, children []*Task) {
	for _, candidate := range tasks {
		if slices.Contains(candidate.BlockedBy, id) {
			dependents = append(dependents, candidate)
		}
		if candidate.ParentTaskID == id {
			children = append(children, candidate)
		}
	}
	slices.SortFunc(dependents, compareByAge)
	slices.SortFunc(children, compareByAge)
	return dependents, children
}

// unlink takes removedID off the blockers and the parent of every linked
// task, one write per task under its own lock. The removal already happened,
// so a task that cannot be written is reported and skipped.
func (service *TaskService) unlink(projectSlug string, removedID TaskID, dependents []*Task, children []*Task) {
	// A task on both lists loses both links in one write.
	linked := slices.Clone(dependents)
	for _, child := range children {
		if !slices.Contains(dependents, child) {
			linked = append(linked, child)
		}
	}

	unblockedCount, ungroupedCount := 0, 0
	for _, linkedTask := range linked {
		wasUnblocked, wasUngrouped := false, false
		isStored, err := service.repo.TryUpdateTask(projectSlug, linkedTask.ID, func(onDisk *Task) error {
			if slices.Contains(onDisk.BlockedBy, removedID) {
				onDisk.BlockedBy = removeBlockers(onDisk.BlockedBy, []TaskID{removedID})
				wasUnblocked = true
			}
			if onDisk.ParentTaskID == removedID {
				onDisk.ParentTaskID = ""
				wasUngrouped = true
			}
			if !wasUnblocked && !wasUngrouped {
				return ErrTaskUnchanged
			}
			return nil
		})
		if err != nil {
			service.log.Error("Task %s is removed, but task %s still names it: %v", removedID, linkedTask.ID, err)
			continue
		}
		if !isStored {
			service.log.Error("Task %s is removed, but another drudge command is working on task %s, which still names it", removedID, linkedTask.ID)
			continue
		}
		if wasUnblocked {
			unblockedCount++
		}
		if wasUngrouped {
			ungroupedCount++
		}
	}

	if unblockedCount > 0 {
		service.log.Info("Took it off the blockers of %s", FormatTaskCount(unblockedCount))
	}
	if ungroupedCount > 0 {
		service.log.Info("Ungrouped %s that belonged to it", FormatTaskCount(ungroupedCount))
	}
}

// approveRemoval puts the removal to the user and returns errRemovalDeclined
// when they turn it down. A forced removal asks nothing.
func approveRemoval(removal Removal, isForced bool, confirm ConfirmRemoval) error {
	if isForced {
		return nil
	}

	isApproved, err := confirm(removal)
	if err != nil {
		return fmt.Errorf("could not read the confirmation for task %s: %w", removal.Task.ID, err)
	}
	if !isApproved {
		return errRemovalDeclined
	}
	return nil
}
