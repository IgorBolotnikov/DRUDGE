package drudger

import (
	"fmt"

	"github.com/IgorBolotnikov/DRUDGE/internal/task"
)

// unblockedTaskLine lays out one task a finished task unblocked.
const unblockedTaskLine = "  %s  %s"

// EditTask changes the fields a user owns on one task, the way
// task.TaskService.EditTask does. An edit that sets the task to done also
// reports the tasks it unblocked.
func (service *DrudgerService) EditTask(projectSlug string, id task.TaskID, changes task.EditTaskDto) (*task.Task, error) {
	edited, err := service.tasks.EditTask(projectSlug, id, changes, service)
	if err != nil {
		return nil, err
	}
	if changes.Status != nil && *changes.Status == task.StatusDone {
		service.reportUnblocked(projectSlug, edited)
	}
	return edited, nil
}

// MarkDone sets an unmerged task to done, the way task.TaskService.MarkDone
// does, and reports the tasks it unblocked.
func (service *DrudgerService) MarkDone(projectSlug string, id task.TaskID) (*task.Task, error) {
	marked, err := service.tasks.MarkDone(projectSlug, id)
	if err != nil {
		return nil, err
	}
	service.reportUnblocked(projectSlug, marked)
	return marked, nil
}

// reportUnblocked names the tasks that became runnable once finished is done.
// It prints nothing when no task became runnable.
//
// The task is stored as done by the time this runs, so a failure is logged
// and the command still succeeds.
func (service *DrudgerService) reportUnblocked(projectSlug string, finished *task.Task) {
	unblocked, err := service.tasks.Unblocked(projectSlug, finished)
	if err != nil {
		service.logger.Error("Task %s is done, but the tasks it unblocked could not be worked out: %v", finished.ID, err)
		return
	}
	if len(unblocked) == 0 {
		return
	}

	lines := []string{fmt.Sprintf("It unblocked %s:", task.FormatTaskCount(len(unblocked)))}
	for _, dependent := range unblocked {
		lines = append(lines, fmt.Sprintf(unblockedTaskLine, task.ShortID(dependent.ID), dependent.Title))
	}

	for _, line := range lines {
		// A task title may hold a percent sign.
		service.logger.Info("%s", line)
	}
}
