package task

import "fmt"

// MarkDone sets an unmerged task to done and returns the task as it stands
// afterwards. The id may be a prefix. A task in any other status is refused,
// and so is one another command holds the lock of.
func (service *TaskService) MarkDone(projectSlug string, id TaskID) (*Task, error) {
	if id == "" {
		return nil, ErrNoTaskID
	}

	found, err := service.repo.FindTask(projectSlug, string(id))
	if err != nil {
		return nil, err
	}

	var marked *Task
	isStored, err := service.repo.TryUpdateTask(projectSlug, found.ID, func(taskToMark *Task) error {
		if taskToMark.Status != StatusUnmerged {
			return fmt.Errorf("task %s is %q, only %q tasks can be marked done, edit its status to mark it done anyway", found.ID, taskToMark.Status, StatusUnmerged)
		}
		taskToMark.Status = StatusDone
		marked = taskToMark
		return nil
	})
	if err != nil {
		return nil, err
	}
	if !isStored {
		return nil, fmt.Errorf("another drudge command is working on task %s, wait for it to finish and run this again", found.ID)
	}

	service.log.Info("Task [%s] %s is %q", marked.ID, marked.Title, marked.Status)
	return marked, nil
}
