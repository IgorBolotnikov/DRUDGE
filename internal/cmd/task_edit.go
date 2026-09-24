package cmd

import (
	"fmt"
	"slices"
	"strings"

	"drudge/internal/task"
)

// editOptionLine lays out one option of the edit help, wide enough for the
// longest flag it lists.
const editOptionLine = "  %-26s %s\n"

// editValueFlags are the flags drg task edit reads a value after.
var editValueFlags = []string{titleFlag, descriptionFlag, ticketFlag, statusFlag, blockedByFlag}

// taskEdit changes the fields a user owns on one task.
func taskEdit(args []string) error {
	if hasFlag(args, helpFlag) || hasFlag(args, helpFlagShort) {
		fmt.Println(taskEditUsage)
		fmt.Println()
		fmt.Println("Change what a task asks for and where it stands. It takes at least one field.")
		fmt.Println("The task ID may be the short one a listing prints, as long as it names a single task.")
		fmt.Println()
		fmt.Println("Options:")
		fmt.Printf(editOptionLine, titleFlag+" <title>", "New title")
		fmt.Printf(editOptionLine, descriptionFlag+" <text>", "New description, the prompt the agent is handed")
		fmt.Printf(editOptionLine, ticketFlag+" <ticket>", "Ticket the task came from, empty to clear it")
		fmt.Printf(editOptionLine, statusFlag+" <status>", "New status ("+task.FormatStatuses(task.Statuses)+")")
		fmt.Printf(editOptionLine, blockedByFlag+" <id>[,<id>...]", "Tasks this task waits for, replacing the list, empty to clear it")
		fmt.Printf(editOptionLine, forceFlag, "Set a status drudge maintains itself ("+task.FormatStatuses(task.ManagedStatuses)+")")
		return nil
	}

	taskID, changes, err := parseTaskEditArgs(args)
	if err != nil {
		return err
	}

	deps, err := newCommandDeps()
	if err != nil {
		return err
	}

	_, err = deps.tasks.EditTask(deps.localCfg.ProjectSlug, taskID, changes, deps.drudger)
	return err
}

// parseTaskEditArgs reads the task id and the fields an edit changes. A flag
// given an empty value clears its field, and a flag left out leaves its field
// as it stands.
func parseTaskEditArgs(args []string) (task.TaskID, task.EditTaskDto, error) {
	var taskID string
	var changes task.EditTaskDto

	for index := 0; index < len(args); index++ {
		arg := args[index]
		switch {
		case arg == forceFlag || arg == forceFlagShort:
			changes.AllowsManagedStatus = true
		case slices.Contains(editValueFlags, arg):
			if index+1 >= len(args) {
				return "", changes, fmt.Errorf("%s needs a value, %s", arg, taskEditUsage)
			}
			index++
			setEditedField(&changes, arg, args[index])
		case strings.HasPrefix(arg, "-"):
			return "", changes, fmt.Errorf("unknown flag %q, %s", arg, taskEditUsage)
		case taskID == "":
			taskID = arg
		default:
			return "", changes, fmt.Errorf("unexpected argument %q, drg task %s takes a single task ID", arg, editSubcommand)
		}
	}

	if taskID == "" {
		return "", changes, fmt.Errorf("task ID is required, %s", taskEditUsage)
	}
	if !changes.HasChanges() {
		return "", changes, fmt.Errorf("nothing to change, %s", taskEditUsage)
	}
	return task.TaskID(taskID), changes, nil
}

// setEditedField puts the value a user gave one flag onto the changes an edit
// carries.
func setEditedField(changes *task.EditTaskDto, flag string, value string) {
	switch flag {
	case titleFlag:
		changes.Title = &value
	case descriptionFlag:
		changes.Description = &value
	case ticketFlag:
		changes.TicketID = &value
	case statusFlag:
		status := task.TaskStatus(value)
		changes.Status = &status
	case blockedByFlag:
		blockedBy := parseTaskIDList(value)
		changes.BlockedBy = &blockedBy
	}
}
