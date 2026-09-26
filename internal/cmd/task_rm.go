package cmd

import (
	"fmt"
	"strings"

	"github.com/IgorBolotnikov/DRUDGE/internal/task"
)

// taskRemove deletes one task and the run directory of its Sessions.
func taskRemove(taskID task.TaskID, isForced bool) error {
	deps, err := newCommandDeps()
	if err != nil {
		return err
	}

	return deps.tasks.RemoveTask(deps.localCfg.ProjectSlug, taskID, isForced, deps.drudger, confirmTaskRemoval)
}

// removalLinkLine lays out one task a removal unblocks or ungroups.
const removalLinkLine = "  %s  %s"

// confirmTaskRemoval asks the user whether a task should go.
func confirmTaskRemoval(removal task.Removal) (bool, error) {
	return ConfirmDeletion(describeRemoval(removal))
}

// describeRemoval names the task a removal deletes and the tasks it unblocks
// and ungroups. A list with no tasks in it is left out.
func describeRemoval(removal task.Removal) string {
	lines := []string{fmt.Sprintf("task %s %q", removal.Task.ID, removal.Task.Title)}

	if count := len(removal.Dependents); count == 1 {
		lines = append(lines, "1 task is blocked by it and will be unblocked:")
	} else if count > 1 {
		lines = append(lines, fmt.Sprintf("%s are blocked by it and will be unblocked:", task.FormatTaskCount(count)))
	}
	for _, dependent := range removal.Dependents {
		lines = append(lines, fmt.Sprintf(removalLinkLine, task.ShortID(dependent.ID), dependent.Title))
	}

	if count := len(removal.Children); count == 1 {
		lines = append(lines, "1 task belongs to it and will be ungrouped:")
	} else if count > 1 {
		lines = append(lines, fmt.Sprintf("%s belong to it and will be ungrouped:", task.FormatTaskCount(count)))
	}
	for _, child := range removal.Children {
		lines = append(lines, fmt.Sprintf(removalLinkLine, task.ShortID(child.ID), child.Title))
	}

	return strings.Join(lines, "\n")
}
