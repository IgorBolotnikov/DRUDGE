package cmd

import (
	"flag"
	"fmt"
	"strings"

	"github.com/IgorBolotnikov/DRUDGE/internal/adapters/persistence"
	"github.com/IgorBolotnikov/DRUDGE/internal/common"
	"github.com/IgorBolotnikov/DRUDGE/internal/config"
	"github.com/IgorBolotnikov/DRUDGE/internal/task"
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

// taskListFlags holds the filters of drg task list. A flag left out filters
// nothing.
type taskListFlags struct {
	status optionalString
	ticket optionalString
	parent optionalString
}

func (flags *taskListFlags) declare(fs *flag.FlagSet) {
	fs.Var(&flags.status, statusFlagName, "Filter by `status` ("+task.FormatStatuses(task.Statuses)+")")
	fs.Var(&flags.ticket, ticketFlagName, "Filter by `ticket` ID")
	fs.Var(&flags.parent, parentFlagName, "Filter by the `id` of the task the tasks belong to")
}

// filter returns the filter of the listing. It refuses a status drudge does
// not know.
func (flags *taskListFlags) filter() (task.ListTasksFilter, error) {
	filter := task.ListTasksFilter{
		Status:   optionalOf[task.TaskStatus](flags.status),
		TicketID: optionalOf[string](flags.ticket),
		ParentID: optionalOf[task.TaskID](flags.parent),
	}
	if filter.Status != nil && !task.KnownStatus(*filter.Status) {
		return task.ListTasksFilter{}, invalidStatusError(*filter.Status)
	}
	return filter, nil
}

func taskList(filter task.ListTasksFilter) error {
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
