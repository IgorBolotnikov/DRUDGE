package task

import (
	"fmt"
	"slices"
)

// ListTasksFilter narrows a listing. A nil field filters nothing, so a field
// set to an empty value keeps the tasks where that field is empty.
type ListTasksFilter struct {
	Status   *TaskStatus
	TicketID *string
	// ParentID may be a prefix of the parent's id.
	ParentID *TaskID
}

func (filter ListTasksFilter) isEmpty() bool {
	return filter.Status == nil && filter.TicketID == nil && filter.ParentID == nil
}

// ListedTask is one row of a listing.
type ListedTask struct {
	Task *Task
	// IsUnderParent is true for a task listed right after its parent.
	IsUnderParent bool
	// Holding are the blockers of Task that are not done or that name no
	// stored task, in the order the task names them.
	Holding []TaskID
}

// ListTasks returns the tasks of a project that pass filter, newest first. An
// unfiltered listing puts the children of a task right after it. A filtered
// listing is flat. It refuses a parent id that names no task or several.
func (service *TaskService) ListTasks(projectSlug string, filter ListTasksFilter) ([]ListedTask, error) {
	if filter.ParentID != nil && *filter.ParentID != "" {
		parent, err := service.repo.FindTask(projectSlug, string(*filter.ParentID))
		if err != nil {
			return nil, fmt.Errorf("could not list the tasks of %q: %w", *filter.ParentID, err)
		}
		filter.ParentID = &parent.ID
	}

	tasks, err := service.repo.ListTasks(projectSlug)
	if err != nil {
		return nil, err
	}
	tasksByID := indexTasks(tasks)

	var kept []*Task
	for _, candidate := range tasks {
		if filter.matches(candidate) {
			kept = append(kept, candidate)
		}
	}
	slices.SortFunc(kept, compareByNewest)

	listRow := func(listedTask *Task, isUnderParent bool) ListedTask {
		return ListedTask{Task: listedTask, IsUnderParent: isUnderParent, Holding: holdingBlockers(tasksByID, listedTask)}
	}

	var listed []ListedTask
	if !filter.isEmpty() {
		for _, listedTask := range kept {
			listed = append(listed, listRow(listedTask, false))
		}
		return listed, nil
	}

	childrenByParent := map[TaskID][]*Task{}
	for _, listedTask := range kept {
		if hasListedParent(tasksByID, listedTask) {
			childrenByParent[listedTask.ParentTaskID] = append(childrenByParent[listedTask.ParentTaskID], listedTask)
		}
	}
	for _, listedTask := range kept {
		if hasListedParent(tasksByID, listedTask) {
			continue
		}
		listed = append(listed, listRow(listedTask, false))
		for _, child := range childrenByParent[listedTask.ID] {
			listed = append(listed, listRow(child, true))
		}
	}
	return listed, nil
}

// hasListedParent reports whether child is listed under its parent. Only a
// parent that belongs to no other task lists children under it, so a chain
// deeper than one level still lists every task.
func hasListedParent(tasksByID map[TaskID]*Task, child *Task) bool {
	parent, ok := tasksByID[child.ParentTaskID]
	return ok && parent.ParentTaskID == ""
}

func (filter ListTasksFilter) matches(candidate *Task) bool {
	if filter.Status != nil && candidate.Status != *filter.Status {
		return false
	}
	if filter.TicketID != nil && candidate.TicketID != *filter.TicketID {
		return false
	}
	if filter.ParentID != nil && candidate.ParentTaskID != *filter.ParentID {
		return false
	}
	return true
}

// compareByNewest orders tasks newest first, and by the highest id among
// tasks made at the same moment.
func compareByNewest(first, second *Task) int {
	return compareByAge(second, first)
}
