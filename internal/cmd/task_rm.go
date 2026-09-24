package cmd

import (
	"fmt"
	"strings"

	"drudge/internal/task"
)

// taskRemove deletes one task and the run directory of its Sessions.
func taskRemove(args []string) error {
	if hasFlag(args, helpFlag) || hasFlag(args, helpFlagShort) {
		fmt.Println(taskRmUsage)
		fmt.Println()
		fmt.Println("Delete a task and the run directory of its Sessions. It asks first.")
		fmt.Println("The branches of the task that hold no commits go with it, and the ones holding work stay.")
		fmt.Println("A task an agent is still working on is refused, so kill its Drudger before removing it.")
		fmt.Println("Tasks blocked by it are unblocked and tasks belonging to it are ungrouped. They stay where they are.")
		fmt.Println("The task ID may be the short one a listing prints, as long as it names a single task.")
		fmt.Println()
		fmt.Println("Options:")
		fmt.Printf("  %s  Remove the task without asking\n", forceFlag)
		return nil
	}

	taskID, isForced, err := parseTaskRemoveArgs(args)
	if err != nil {
		return err
	}

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

// parseTaskRemoveArgs reads the task id and the force flag a removal takes.
func parseTaskRemoveArgs(args []string) (task.TaskID, bool, error) {
	var taskID string
	isForced := false

	for _, arg := range args {
		switch {
		case arg == forceFlag || arg == forceFlagShort:
			isForced = true
		case strings.HasPrefix(arg, "-"):
			return "", false, fmt.Errorf("unknown flag %q, %s", arg, taskRmUsage)
		case taskID == "":
			taskID = arg
		default:
			return "", false, fmt.Errorf("unexpected argument %q, drg task %s takes a single task ID", arg, rmSubcommand)
		}
	}

	if taskID == "" {
		return "", false, fmt.Errorf("task ID is required, %s", taskRmUsage)
	}

	return task.TaskID(taskID), isForced, nil
}
