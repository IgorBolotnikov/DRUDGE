package task

import (
	"errors"
	"fmt"
	"slices"
	"strings"
)

// ErrNoChanges reports an edit that names no field to change.
var ErrNoChanges = errors.New("nothing to change, name at least one of the title, the description, the ticket, the status and the blockers")

// ManagedStatuses are the statuses that describe a Session. Drudge writes them
// itself when a run starts and when it ends.
var ManagedStatuses = []TaskStatus{StatusInProgress, StatusFuckedUp, StatusDone}

// EditTaskDto carries the fields a user may change on a task. A nil field is
// left as it stands, so a field a user names is set even when it is empty.
type EditTaskDto struct {
	Title       *string
	Description *string
	TicketID    *string
	Status      *TaskStatus
	// BlockedBy replaces the whole list of blockers. The ids may be prefixes,
	// and an empty list clears it.
	BlockedBy *[]TaskID
	// Block adds IDs to the list of blockers.
	Block *[]TaskID
	// Unblock removes the IDs from the list of blockers.
	Unblock *[]TaskID

	// AllowsManagedStatus lets the edit set one of ManagedStatuses. The CLI
	// reads it off the force flag.
	AllowsManagedStatus bool
}

// HasChanges reports whether the edit names a field to change.
func (changes EditTaskDto) HasChanges() bool {
	return changes.Title != nil || changes.Description != nil || changes.TicketID != nil || changes.Status != nil ||
		changes.BlockedBy != nil || changes.Block != nil || changes.Unblock != nil
}

// SessionGuard refuses a change to a task whose agent is still working.
// Telling a live Session apart from a finished one means reading the run
// directory and the Drudgers of the project, which the drudger service does.
type SessionGuard interface {
	RefuseWhileWorking(projectSlug string, taskToChange *Task) error
}

// EditTask changes the fields a user owns on one task and returns the task as
// it stands afterwards. The id may be a prefix. sessions runs against the
// stored task under the lock, which is what catches a task an agent picked up
// since the lookup. An edit that finds the lock taken writes nothing.
func (service *TaskService) EditTask(projectSlug string, id TaskID, changes EditTaskDto, sessions SessionGuard) (*Task, error) {
	if id == "" {
		return nil, ErrNoTaskID
	}
	if err := validateEdit(changes); err != nil {
		return nil, err
	}

	found, err := service.repo.FindTask(projectSlug, string(id))
	if err != nil {
		return nil, err
	}

	if changes.BlockedBy != nil {
		blockedBy, err := service.resolveBlockers(projectSlug, found.ID, *changes.BlockedBy)
		if err != nil {
			return nil, err
		}
		changes.BlockedBy = &blockedBy
	}
	if changes.Block != nil {
		block, err := service.resolveBlockers(projectSlug, found.ID, *changes.Block)
		if err != nil {
			return nil, err
		}
		changes.Block = &block
	}
	if changes.Unblock != nil {
		unblock, err := service.resolveUnblocked(projectSlug, found, *changes.Unblock)
		if err != nil {
			return nil, err
		}
		changes.Unblock = &unblock
	}

	var edited *Task
	isStored, err := service.repo.TryUpdateTask(projectSlug, found.ID, func(taskToEdit *Task) error {
		if err := sessions.RefuseWhileWorking(projectSlug, taskToEdit); err != nil {
			return err
		}
		applyEdit(taskToEdit, changes)
		edited = taskToEdit
		return nil
	})
	if err != nil {
		return nil, err
	}
	if !isStored {
		return nil, fmt.Errorf("another drudge command is working on task %s, wait for it to finish and run this again", found.ID)
	}

	service.log.Info("Updated task [%s] %s, it is now %q", edited.ID, edited.Title, edited.Status)
	return edited, nil
}

// validateEdit refuses an edit before it reaches the stored task.
func validateEdit(changes EditTaskDto) error {
	if !changes.HasChanges() {
		return ErrNoChanges
	}

	if changes.Title != nil && strings.TrimSpace(*changes.Title) == "" {
		return errors.New("a task title cannot be blank")
	}

	if err := validateBlockerEdit(changes); err != nil {
		return err
	}

	if changes.Status == nil {
		return nil
	}

	wanted := *changes.Status
	if !KnownStatus(wanted) {
		return fmt.Errorf("invalid status %q, must be one of: %s", wanted, FormatStatuses(Statuses))
	}
	if slices.Contains(ManagedStatuses, wanted) && !changes.AllowsManagedStatus {
		return fmt.Errorf("status %q is one drudge writes itself when a Session starts and ends, force the edit to set it by hand", wanted)
	}
	return nil
}

// applyEdit puts the fields an edit names onto a task.
func applyEdit(taskToEdit *Task, changes EditTaskDto) {
	if changes.Title != nil {
		taskToEdit.Title = *changes.Title
	}
	if changes.Description != nil {
		taskToEdit.Description = *changes.Description
	}
	if changes.TicketID != nil {
		taskToEdit.TicketID = *changes.TicketID
	}
	if changes.Status != nil {
		taskToEdit.Status = *changes.Status
	}
	if changes.BlockedBy != nil {
		taskToEdit.BlockedBy = *changes.BlockedBy
	}
	if changes.Block != nil {
		taskToEdit.BlockedBy = addBlockers(taskToEdit.BlockedBy, *changes.Block)
	}
	if changes.Unblock != nil {
		taskToEdit.BlockedBy = removeBlockers(taskToEdit.BlockedBy, *changes.Unblock)
	}
}
