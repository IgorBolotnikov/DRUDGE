package task

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"drudge/internal/common"
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

// Unblocked returns the todo tasks blocked by finished whose other blockers
// are all done, oldest first and the lowest id first among tasks made at the
// same moment. finished counts as done whatever its stored status says.
func (service *TaskService) Unblocked(projectSlug string, finished *Task) ([]*Task, error) {
	tasks, err := service.repo.ListTasks(projectSlug)
	if err != nil {
		return nil, fmt.Errorf("could not read the tasks blocked by task %s: %w", finished.ID, err)
	}
	tasksByID := indexTasks(tasks)

	var unblocked []*Task
	for _, dependent := range tasks {
		if dependent.Status == StatusTodo && slices.Contains(dependent.BlockedBy, finished.ID) && isOnlyBlockedBy(tasksByID, dependent, finished.ID) {
			unblocked = append(unblocked, dependent)
		}
	}
	slices.SortFunc(unblocked, func(first, second *Task) int {
		if byAge := first.CreatedAt.Compare(second.CreatedAt); byAge != 0 {
			return byAge
		}
		return strings.Compare(string(first.ID), string(second.ID))
	})
	return unblocked, nil
}

// isOnlyBlockedBy reports whether every blocker of dependent other than
// finishedID is a stored task that is done.
func isOnlyBlockedBy(tasksByID map[TaskID]*Task, dependent *Task, finishedID TaskID) bool {
	for _, blockerID := range dependent.BlockedBy {
		if blockerID == finishedID {
			continue
		}
		blocker, ok := tasksByID[blockerID]
		if !ok || blocker.Status != StatusDone {
			return false
		}
	}
	return true
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

// resolveUnblocked turns the ids a user named into the full ids of blockers
// dependent carries. An id is first matched against those blockers, which
// lets a user drop a blocker whose task is gone. It refuses an id that names
// no task or several, and a task dependent is not blocked by.
func (service *TaskService) resolveUnblocked(projectSlug string, dependent *Task, ids []TaskID) ([]TaskID, error) {
	var resolved []TaskID
	for _, id := range ids {
		matches := slices.DeleteFunc(slices.Clone(dependent.BlockedBy), func(blockerID TaskID) bool {
			return !strings.HasPrefix(string(blockerID), string(id))
		})
		if len(matches) == 1 {
			resolved = append(resolved, matches[0])
			continue
		}

		found, err := service.repo.FindTask(projectSlug, string(id))
		if err != nil {
			return nil, fmt.Errorf("could not unblock %q: %w", id, err)
		}
		return nil, fmt.Errorf("task %s is not blocked by %s %s", ShortID(dependent.ID), ShortID(found.ID), found.Title)
	}
	return resolved, nil
}

// validateBlockerEdit refuses an edit that changes the blockers in more than
// one way, or that adds or removes an empty list.
func validateBlockerEdit(changes EditTaskDto) error {
	var named []string
	if changes.BlockedBy != nil {
		named = append(named, "the blocker list")
	}
	if changes.Block != nil {
		named = append(named, "the blockers to add")
	}
	if changes.Unblock != nil {
		named = append(named, "the blockers to remove")
	}
	if len(named) > 1 {
		return fmt.Errorf("an edit changes the blockers one way at a time, got %s", common.JoinNames(named))
	}

	if changes.Block != nil && len(*changes.Block) == 0 {
		return errors.New("name at least one task to add as a blocker")
	}
	if changes.Unblock != nil && len(*changes.Unblock) == 0 {
		return errors.New("name at least one task to remove from the blockers")
	}
	return nil
}

// addBlockers appends the ids blockedBy does not hold yet.
func addBlockers(blockedBy []TaskID, added []TaskID) []TaskID {
	for _, id := range added {
		if !slices.Contains(blockedBy, id) {
			blockedBy = append(blockedBy, id)
		}
	}
	return blockedBy
}

// removeBlockers returns blockedBy without the removed ids, and nil when none
// are left.
func removeBlockers(blockedBy []TaskID, removed []TaskID) []TaskID {
	var kept []TaskID
	for _, id := range blockedBy {
		if !slices.Contains(removed, id) {
			kept = append(kept, id)
		}
	}
	return kept
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
