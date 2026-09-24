package drudger

import (
	"fmt"

	"drudge/internal/task"
)

// unblockedTaskLine lays out one task a finished task unblocked.
const unblockedTaskLine = "  %s  %s"

// unmergedWorkLineIndent sets the unmerged work of a finished task under the
// heading that names it.
const unmergedWorkLineIndent = "  "

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

// reportUnblocked names the tasks that became runnable once finished is done,
// and the work of finished that has not reached its base yet. It prints
// nothing when no task became runnable.
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

	lines := []string{fmt.Sprintf("It unblocked %s:", formatTaskCount(len(unblocked)))}
	for _, dependent := range unblocked {
		lines = append(lines, fmt.Sprintf(unblockedTaskLine, task.ShortID(dependent.ID), dependent.Title))
	}

	unmerged, err := service.UnmergedWork([]task.Blocker{{ID: finished.ID, Task: finished}})
	if err != nil {
		service.logger.Error("Could not tell whether the work of task %s is merged: %v", finished.ID, err)
	}
	if work := unmerged[finished.ID]; len(work) > 0 {
		lines = append(lines, mergeHeading(len(unblocked)))
		for _, line := range FormatUnmergedWork(work) {
			lines = append(lines, unmergedWorkLineIndent+line)
		}
	}

	for _, line := range lines {
		// A task title may hold a percent sign.
		service.logger.Info("%s", line)
	}
}

func formatTaskCount(count int) string {
	if count == 1 {
		return "1 task"
	}
	return fmt.Sprintf("%d tasks", count)
}

func mergeHeading(unblockedCount int) string {
	if unblockedCount == 1 {
		return "That task needs its work merged first:"
	}
	return "They need its work merged first:"
}
