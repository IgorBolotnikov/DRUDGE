package cmd

import (
	"fmt"
	"strings"

	"drudge/internal/adapters/persistence"
	"drudge/internal/common"
	"drudge/internal/config"
	"drudge/internal/task"
)

// taskTitleWidth is how much room a listing gives a task title.
const taskTitleWidth = 40

// taskStatusWidth is how much room a listing gives a task status.
const taskStatusWidth = 15

// blockedByTitle heads the column of blockers holding a task.
const blockedByTitle = "BLOCKED BY"

// childRowIndent starts the status and the title of a task listed under its
// parent.
const childRowIndent = "  "

func taskList(args []string) error {
	// TODO: make a util for printing out help text
	if hasFlag(args, helpFlag) || hasFlag(args, helpFlagShort) {
		fmt.Println(taskListUsage)
		fmt.Println()
		fmt.Println("List tasks in the current project. Tasks belonging to another task are listed under it.")
		fmt.Println("A filter lists the tasks it keeps without grouping them.")
		fmt.Println()
		fmt.Println("Options:")
		fmt.Printf("  %s <status>  Filter by status (%s)\n", statusFlag, task.FormatStatuses(task.Statuses))
		fmt.Printf("  %s <ticket>  Filter by ticket ID\n", ticketFlag)
		fmt.Printf("  %s <id>      Filter by the task the tasks belong to\n", parentFlag)
		return nil
	}

	filter, err := parseTaskListArgs(args)
	if err != nil {
		return err
	}

	cfg, err := config.LoadLocal()
	if err != nil {
		return err
	}

	log := common.NewLogger("")
	repo := persistence.NewFileTaskRepository(cfg.ProjectSlug)
	svc := task.NewTaskService(repo, log)

	listed, err := svc.ListTasks(cfg.ProjectSlug, filter)
	if err != nil {
		return fmt.Errorf("could not list tasks: %w", err)
	}

	printTaskList(log, listed)
	return nil
}

// parseTaskListArgs reads the filters of drg task list. A flag left out
// filters nothing.
func parseTaskListArgs(args []string) (task.ListTasksFilter, error) {
	var filter task.ListTasksFilter
	if statusValue, hasStatus := parseFlagValue(args, statusFlag); hasStatus {
		status := task.TaskStatus(statusValue)
		if !task.KnownStatus(status) {
			return task.ListTasksFilter{}, invalidStatusError(status)
		}
		filter.Status = &status
	}
	if ticketID, hasTicket := parseFlagValue(args, ticketFlag); hasTicket {
		filter.TicketID = &ticketID
	}
	if parentID, hasParent := parseFlagValue(args, parentFlag); hasParent {
		id := task.TaskID(parentID)
		filter.ParentID = &id
	}
	return filter, nil
}

// printTaskList prints a listing, indenting the tasks listed under a parent.
// The blockers column is as wide as its widest value.
func printTaskList(log *common.Logger, listed []task.ListedTask) {
	if len(listed) == 0 {
		log.Info("No tasks found")
		return
	}

	blockedByWidth := len(blockedByTitle)
	rows := make([][]string, 0, len(listed))
	for _, entry := range listed {
		holding := make([]string, 0, len(entry.Holding))
		for _, id := range entry.Holding {
			holding = append(holding, task.ShortID(id))
		}
		blockedBy := strings.Join(holding, holdingBlockerSeparator)
		blockedByWidth = max(blockedByWidth, len(blockedBy))

		indent := ""
		if entry.IsUnderParent {
			indent = childRowIndent
		}
		rows = append(rows, []string{
			indent + string(entry.Task.Status),
			task.ShortID(entry.Task.ID),
			indent + entry.Task.Title,
			blockedBy,
			entry.Task.TicketID,
		})
	}

	columns := []column{
		{Title: "STATUS", Width: taskStatusWidth},
		{Title: "ID", Width: task.ShortIDLength},
		{Title: "TITLE", Width: taskTitleWidth},
		{Title: blockedByTitle, Width: blockedByWidth},
		{Title: "TICKET"},
	}
	printList(log, "Tasks", columns, rows)
}
