package task

import (
	"fmt"
	"slices"
	"strings"
)

// Pick is the task that can be started now. When no task can, it holds the
// todo tasks that are blocked.
type Pick struct {
	// Task is nil when no todo task has every blocker done.
	Task *Task
	// Blockers are the blockers of Task. All of them are done.
	Blockers []Blocker
	// Blocked is filled only when Task is nil.
	Blocked []BlockedTask
}

// BlockedTask is a todo task and the blockers holding it back.
type BlockedTask struct {
	Task *Task
	// Holding are the ids of the blockers that are not done or that name no
	// stored task, in the order the task names them.
	Holding []TaskID
}

// NextTask picks the oldest todo task whose blockers are all done, and the
// lowest id among tasks made at the same moment. Unmerged work of a blocker
// does not hold a task back here. When no task qualifies, the pick lists every
// todo task with the blockers holding it, oldest first.
func (service *TaskService) NextTask(projectSlug string) (Pick, error) {
	tasks, err := service.repo.ListTasks(projectSlug)
	if err != nil {
		return Pick{}, fmt.Errorf("could not read the tasks to pick from: %w", err)
	}
	tasksByID := indexTasks(tasks)

	todo := slices.DeleteFunc(slices.Clone(tasks), func(candidate *Task) bool {
		return candidate.Status != StatusTodo
	})
	slices.SortFunc(todo, compareByAge)

	var blocked []BlockedTask
	for _, candidate := range todo {
		holding := holdingBlockers(tasksByID, candidate)
		if len(holding) > 0 {
			blocked = append(blocked, BlockedTask{Task: candidate, Holding: holding})
			continue
		}

		var blockers []Blocker
		for _, id := range candidate.BlockedBy {
			blockers = append(blockers, Blocker{ID: id, Task: tasksByID[id]})
		}
		return Pick{Task: candidate, Blockers: blockers}, nil
	}
	return Pick{Blocked: blocked}, nil
}

// holdingBlockers returns the blockers of dependent that are not a stored task
// that is done.
func holdingBlockers(tasksByID map[TaskID]*Task, dependent *Task) []TaskID {
	var holding []TaskID
	for _, blockerID := range dependent.BlockedBy {
		blocker, ok := tasksByID[blockerID]
		if !ok || blocker.Status != StatusDone {
			holding = append(holding, blockerID)
		}
	}
	return holding
}

// compareByAge orders tasks oldest first, and by the lowest id among tasks
// made at the same moment.
func compareByAge(first, second *Task) int {
	if byAge := first.CreatedAt.Compare(second.CreatedAt); byAge != 0 {
		return byAge
	}
	return strings.Compare(string(first.ID), string(second.ID))
}
