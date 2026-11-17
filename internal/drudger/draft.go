package drudger

import (
	"github.com/IgorBolotnikov/DRUDGE/internal/task"
)

// EditTaskFromDraft changes a task to what its draft describes the way
// EditTask does, and removes the draft. It returns the task as it stands and
// the fields of the draft that changed. A draft that changes nothing is
// removed and returns task.ErrDraftUnchanged. A draft that does not parse and
// an edit the task service refuses keep the draft. A draft that cannot be
// removed after the edit is reported as a warning event.
func (service *DrudgerService) EditTaskFromDraft(projectSlug string, id task.TaskID) (*task.Task, []string, error) {
	stored, draft, err := service.tasks.ReadTaskDraft(projectSlug, id)
	if err != nil {
		return nil, nil, err
	}
	changes := task.DiffDraft(stored, draft)
	if !changes.HasChanges() {
		if err := service.tasks.RemoveTaskDraft(projectSlug, stored.ID); err != nil {
			return nil, nil, err
		}
		return nil, nil, task.ErrDraftUnchanged
	}
	edited, err := service.EditTask(projectSlug, stored.ID, changes)
	if err != nil {
		return nil, nil, err
	}
	if err := service.tasks.RemoveTaskDraft(projectSlug, stored.ID); err != nil {
		service.progress.Report(task.EditDraftRemovalFailed{Task: edited, Err: err})
	}
	return edited, changes.DraftFields(), nil
}
