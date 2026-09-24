package task

import "slices"

// ListTasksFilter narrows a listing. A nil field filters nothing, so a field
// set to an empty value keeps the tasks where that field is empty.
type ListTasksFilter struct {
	Status   *TaskStatus
	TicketID *string
}

// ListTasks returns the tasks of a project that pass filter, newest first.
func (service *TaskService) ListTasks(projectSlug string, filter ListTasksFilter) ([]*Task, error) {
	tasks, err := service.repo.ListTasks(projectSlug)
	if err != nil {
		return nil, err
	}

	var listed []*Task
	for _, candidate := range tasks {
		if filter.matches(candidate) {
			listed = append(listed, candidate)
		}
	}
	slices.SortFunc(listed, func(first, second *Task) int {
		return second.CreatedAt.Compare(first.CreatedAt)
	})
	return listed, nil
}

func (filter ListTasksFilter) matches(candidate *Task) bool {
	if filter.Status != nil && candidate.Status != *filter.Status {
		return false
	}
	if filter.TicketID != nil && candidate.TicketID != *filter.TicketID {
		return false
	}
	return true
}
