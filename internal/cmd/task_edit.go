package cmd

import (
	"fmt"
	"io"
	"os"
	"slices"
	"strings"

	"drudge/internal/common"
	"drudge/internal/task"
)

// editValueFlags are the flags drg task edit reads a value after.
var editValueFlags = []string{titleFlag, descriptionFlag, ticketFlag, statusFlag, blockedByFlag, blockFlag, unblockFlag, parentFlag}

// blockerFlags are the flags that change the blockers of a task. An edit takes
// one of them.
var blockerFlags = []string{blockedByFlag, blockFlag, unblockFlag}

// taskEdit changes the fields a user owns on one task.
func taskEdit(args []string) error {
	if hasFlag(args, helpFlag) || hasFlag(args, helpFlagShort) {
		fmt.Println(taskEditUsage)
		fmt.Println()
		fmt.Println("Change what a task asks for and where it stands. It takes at least one field.")
		fmt.Println("The task ID may be the short one a listing prints, as long as it names a single task.")
		fmt.Println()
		fmt.Println("Options:")
		fmt.Printf(taskOptionLine, titleFlag+" <title>", "New title")
		fmt.Printf(taskOptionLine, descriptionFlag+" <text>", "New description, the prompt the agent is handed")
		fmt.Printf(taskOptionLine, descriptionFileFlag+" <path>", "File to read the new description from, "+stdinPath+" to read stdin")
		fmt.Printf(taskOptionLine, ticketFlag+" <ticket>", "Ticket the task came from, empty to clear it")
		fmt.Printf(taskOptionLine, statusFlag+" <status>", "New status ("+task.FormatStatuses(task.Statuses)+")")
		fmt.Printf(taskOptionLine, blockedByFlag+" <id>[,<id>...]", "Tasks this task waits for, replacing the list, empty to clear it")
		fmt.Printf(taskOptionLine, blockFlag+" <id>[,<id>...]", "Tasks to add to the ones this task waits for")
		fmt.Printf(taskOptionLine, unblockFlag+" <id>[,<id>...]", "Tasks to remove from the ones this task waits for")
		fmt.Printf(taskOptionLine, parentFlag+" <id>", "Task this task belongs to, empty to ungroup it")
		fmt.Printf(taskOptionLine, forceFlag, "Set a status drudge maintains itself ("+task.FormatStatuses(task.ManagedStatuses)+")")
		return nil
	}

	taskID, changes, err := parseTaskEditArgs(args, os.Stdin)
	if err != nil {
		return err
	}

	deps, err := newCommandDeps()
	if err != nil {
		return err
	}

	_, err = deps.drudger.EditTask(deps.localCfg.ProjectSlug, taskID, changes)
	return err
}

// parseTaskEditArgs reads the task id and the fields an edit changes. A flag
// given an empty value clears its field, and a flag left out leaves its field
// as it stands.
func parseTaskEditArgs(args []string, stdin io.Reader) (task.TaskID, task.EditTaskDto, error) {
	var taskID string
	var changes task.EditTaskDto
	var seenBlockerFlags []string
	var descriptionPath string
	var hasDescPath bool

	for index := 0; index < len(args); index++ {
		arg := args[index]
		switch {
		case arg == forceFlag || arg == forceFlagShort:
			changes.AllowsManagedStatus = true
		case arg == descriptionFileFlag:
			if index+1 >= len(args) {
				return "", changes, errDescriptionFileNeedsPath
			}
			index++
			descriptionPath, hasDescPath = args[index], true
		case slices.Contains(editValueFlags, arg):
			if index+1 >= len(args) {
				return "", changes, fmt.Errorf("%s needs a value, %s", arg, taskEditUsage)
			}
			if slices.Contains(blockerFlags, arg) && !slices.Contains(seenBlockerFlags, arg) {
				seenBlockerFlags = append(seenBlockerFlags, arg)
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
	if len(seenBlockerFlags) > 1 {
		return "", changes, fmt.Errorf("%s cannot be used together, change the blockers one way per edit", common.JoinNames(seenBlockerFlags))
	}
	if hasDescPath {
		if changes.Description != nil {
			return "", changes, errTwoDescriptions
		}
		description, err := readDescriptionFile(descriptionPath, stdin)
		if err != nil {
			return "", changes, err
		}
		changes.Description = &description
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
	case blockFlag:
		block := parseTaskIDList(value)
		changes.Block = &block
	case unblockFlag:
		unblock := parseTaskIDList(value)
		changes.Unblock = &unblock
	case parentFlag:
		parentID := task.TaskID(value)
		changes.ParentTaskID = &parentID
	}
}
