package drudger

import (
	"github.com/IgorBolotnikov/DRUDGE/internal/task"
)

// DependentsUnblocked reports the tasks that became runnable once a task was
// done.
type DependentsUnblocked struct {
	Tasks []*task.Task
}

// UnblockedLookupFailed reports a task stored as done whose unblocked tasks
// could not be worked out.
type UnblockedLookupFailed struct {
	TaskID task.TaskID
	Err    error
}

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

// reportUnblocked reports the tasks that became runnable once finished is
// done. It reports nothing when no task became runnable.
//
// The task is stored as done by the time this runs, so a failure is reported
// and the command still succeeds.
func (service *DrudgerService) reportUnblocked(projectSlug string, finished *task.Task) {
	unblocked, err := service.tasks.Unblocked(projectSlug, finished)
	if err != nil {
		service.progress.Report(UnblockedLookupFailed{TaskID: finished.ID, Err: err})
		return
	}
	if len(unblocked) == 0 {
		return
	}
	service.progress.Report(DependentsUnblocked{Tasks: unblocked})
}
