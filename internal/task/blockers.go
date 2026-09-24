package task

import (
	"fmt"
	"slices"
	"strings"
)

// Blocker is one task another task waits for. Task is nil when the id names no
// stored task.
type Blocker struct {
	ID   TaskID
	Task *Task
}

// Blockers reads the tasks a task is blocked by, in the order the task names
// them.
func (service *TaskService) Blockers(projectSlug string, dependent *Task) ([]Blocker, error) {
	blockers := make([]Blocker, 0, len(dependent.BlockedBy))
	if len(dependent.BlockedBy) == 0 {
		return blockers, nil
	}

	tasks, err := service.repo.ListTasks(projectSlug)
	if err != nil {
		return nil, fmt.Errorf("could not read the blockers of task %s: %w", dependent.ID, err)
	}
	tasksByID := indexTasks(tasks)

	for _, id := range dependent.BlockedBy {
		blockers = append(blockers, Blocker{ID: id, Task: tasksByID[id]})
	}
	return blockers, nil
}

// resolveBlockers turns the ids a user named into the full ids of the tasks
// they name, with repeats dropped. dependentID is the task being blocked, and
// is empty for a task not created yet. It refuses an id that names no task or
// several, and a blocker whose chain of blockers reaches the dependent.
func (service *TaskService) resolveBlockers(projectSlug string, dependentID TaskID, ids []TaskID) ([]TaskID, error) {
	var resolved []TaskID
	for _, id := range ids {
		found, err := service.repo.FindTask(projectSlug, string(id))
		if err != nil {
			return nil, fmt.Errorf("could not block on %q: %w", id, err)
		}
		if !slices.Contains(resolved, found.ID) {
			resolved = append(resolved, found.ID)
		}
	}

	// No stored task names a task that is not created yet as its blocker, so a
	// new task cannot close a cycle.
	if dependentID == "" || len(resolved) == 0 {
		return resolved, nil
	}

	tasks, err := service.repo.ListTasks(projectSlug)
	if err != nil {
		return nil, fmt.Errorf("could not check the blockers for a cycle: %w", err)
	}
	tasksByID := indexTasks(tasks)

	for _, blockerID := range resolved {
		if path := blockingPath(tasksByID, blockerID, dependentID); path != nil {
			return nil, cycleError(tasksByID, dependentID, path)
		}
	}
	return resolved, nil
}

// blockingPath follows BlockedBy from start and returns the chain of tasks
// that reaches target, start first. It returns nil when no chain reaches
// target. The visited set stops the walk on a cycle that target is not part
// of.
func blockingPath(tasksByID map[TaskID]*Task, start TaskID, target TaskID) []TaskID {
	visited := map[TaskID]bool{}

	var walk func(id TaskID) []TaskID
	walk = func(id TaskID) []TaskID {
		if id == target {
			return []TaskID{id}
		}
		if visited[id] {
			return nil
		}
		visited[id] = true

		current, ok := tasksByID[id]
		if !ok {
			return nil
		}
		for _, next := range current.BlockedBy {
			if rest := walk(next); rest != nil {
				return append([]TaskID{id}, rest...)
			}
		}
		return nil
	}

	return walk(start)
}

// cycleError refuses a blocker whose chain reaches the dependent and prints
// that chain, one task per line.
func cycleError(tasksByID map[TaskID]*Task, dependentID TaskID, path []TaskID) error {
	lines := make([]string, 0, len(path))
	for _, id := range path {
		title := ""
		if found, ok := tasksByID[id]; ok {
			title = found.Title
		}
		lines = append(lines, fmt.Sprintf("  %s  %s", ShortID(id), title))
	}
	return fmt.Errorf(
		"task %s cannot be blocked by %s, that would make a cycle:\n%s",
		ShortID(dependentID), ShortID(path[0]), strings.Join(lines, "\n"),
	)
}

func indexTasks(tasks []*Task) map[TaskID]*Task {
	tasksByID := make(map[TaskID]*Task, len(tasks))
	for _, candidate := range tasks {
		tasksByID[candidate.ID] = candidate
	}
	return tasksByID
}
