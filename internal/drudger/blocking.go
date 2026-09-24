package drudger

import (
	"fmt"
	"strings"

	"drudge/internal/task"
)

// unfinishedBlockerLine lays out one blocker in a refusal, the status padded
// to the longest status there is.
const unfinishedBlockerLine = "  %s  %-11s  %s"

// missingBlockerLine lays out a blocker id that names no stored task.
const missingBlockerLine = "  %s  no such task"

// refuseBlocked refuses a task with a blocker that is not done or whose id
// names no stored task. The refusal lists every such blocker.
func (service *DrudgerService) refuseBlocked(projectSlug string, dependent *task.Task) error {
	blockers, err := service.tasks.Blockers(projectSlug, dependent)
	if err != nil {
		return err
	}

	var lines []string
	for _, blocker := range blockers {
		switch {
		case blocker.Task == nil:
			lines = append(lines, fmt.Sprintf(missingBlockerLine, task.ShortID(blocker.ID)))
		case blocker.Task.Status != task.StatusDone:
			lines = append(lines, fmt.Sprintf(unfinishedBlockerLine, task.ShortID(blocker.ID), blocker.Task.Status, blocker.Task.Title))
		}
	}
	if len(lines) == 0 {
		return nil
	}
	return fmt.Errorf("task %s is blocked by:\n%s", task.ShortID(dependent.ID), strings.Join(lines, "\n"))
}
