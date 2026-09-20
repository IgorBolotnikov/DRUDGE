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
		fmt.Println("A task an agent is still working on is refused, so kill its Drudger before removing it.")
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

// confirmTaskRemoval asks the user whether a task should go.
func confirmTaskRemoval(taskToRemove *task.Task) (bool, error) {
	return ConfirmDeletion(fmt.Sprintf("task %s %q", taskToRemove.ID, taskToRemove.Title))
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
