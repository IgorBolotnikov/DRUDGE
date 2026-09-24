package task

import (
	"fmt"
	"slices"
)

// Family is the parent and the children of one task.
type Family struct {
	ParentID TaskID // Empty for a task that belongs to no other task
	Parent   *Task  // nil when ParentID names no stored task
	Children []*Task
}

// Family reads the parent of member and the tasks belonging to member, oldest
// first.
func (service *TaskService) Family(projectSlug string, member *Task) (Family, error) {
	tasks, err := service.repo.ListTasks(projectSlug)
	if err != nil {
		return Family{}, fmt.Errorf("could not read the parent and children of task %s: %w", member.ID, err)
	}

	family := Family{ParentID: member.ParentTaskID}
	for _, candidate := range tasks {
		if member.ParentTaskID != "" && candidate.ID == member.ParentTaskID {
			family.Parent = candidate
		}
		if candidate.ParentTaskID == member.ID {
			family.Children = append(family.Children, candidate)
		}
	}
	slices.SortFunc(family.Children, compareByAge)
	return family, nil
}

// resolveParent turns the id a user named into the full id of the parent task.
// childID is the task being grouped, and is empty for a task not created yet.
// An empty id resolves to no parent. It refuses an id that names no task or
// several, the child itself, a parent that belongs to another task and a child
// that has children, which keeps grouping one level deep.
func (service *TaskService) resolveParent(projectSlug string, childID TaskID, id TaskID) (TaskID, error) {
	if id == "" {
		return "", nil
	}

	parent, err := service.repo.FindTask(projectSlug, string(id))
	if err != nil {
		return "", fmt.Errorf("could not group under %q: %w", id, err)
	}
	if parent.ID == childID {
		return "", fmt.Errorf("task %s cannot be its own parent", ShortID(childID))
	}
	if parent.ParentTaskID != "" {
		return "", fmt.Errorf(
			"task %s %s belongs to %s, tasks group one level deep",
			ShortID(parent.ID), parent.Title, ShortID(parent.ParentTaskID),
		)
	}

	// No stored task names a task that is not created yet as its parent.
	if childID == "" {
		return parent.ID, nil
	}

	tasks, err := service.repo.ListTasks(projectSlug)
	if err != nil {
		return "", fmt.Errorf("could not check task %s for children: %w", ShortID(childID), err)
	}
	for _, candidate := range tasks {
		if candidate.ParentTaskID == childID {
			return "", fmt.Errorf("task %s has tasks belonging to it, tasks group one level deep", ShortID(childID))
		}
	}
	return parent.ID, nil
}
