package cmd

import (
	"flag"
	"strings"

	"github.com/IgorBolotnikov/DRUDGE/internal/adapters/persistence"
	"github.com/IgorBolotnikov/DRUDGE/internal/common"
	"github.com/IgorBolotnikov/DRUDGE/internal/config"
	"github.com/IgorBolotnikov/DRUDGE/internal/task"
	"github.com/IgorBolotnikov/DRUDGE/internal/theme"
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

// statusRoles maps a task status to the theme role it prints in. A status
// left out, like todo, prints plain.
var statusRoles = map[task.TaskStatus]string{
	task.StatusDraft:      theme.RoleMuted,
	task.StatusInProgress: theme.RoleInfo,
	task.StatusFuckedUp:   theme.RoleError,
	task.StatusUnmerged:   theme.RoleWarning,
	task.StatusDone:       theme.RoleSuccess,
}

// loadStatusColor returns what colors a task status in the loaded theme.
func loadStatusColor(log *common.Logger) func(text string) string {
	return loadRoleColor(log, "task statuses", statusRoles)
}

// taskListFlags holds the filters and the page of drg task list. A filter
// left out filters nothing.
type taskListFlags struct {
	status optionalString
	ticket optionalString
	parent optionalString
	page   pageFlags
}

func (flags *taskListFlags) declare(fs *flag.FlagSet) {
	fs.Var(&flags.status, statusFlagName, "Filter by `status` ("+task.FormatStatuses(task.Statuses)+")")
	fs.Var(&flags.ticket, ticketFlagName, "Filter by `ticket` ID")
	fs.Var(&flags.parent, parentFlagName, "Filter by the `id` of the task the tasks belong to")
	flags.page.declare(fs, config.TaskPageSizeKey+" from the project config or the global config")
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

func taskList(filter task.ListTasksFilter, page pageFlags) error {
	cfg, err := config.LoadLocal()
	if err != nil {
		return err
	}
	globalCfg, err := config.Load()
	if err != nil {
		return err
	}
	size, err := page.pageSize(config.ResolveTaskPageSize(cfg, globalCfg))
	if err != nil {
		return err
	}

	log := newLogger()
	repo := persistence.NewFileTaskRepository(cfg.ProjectSlug)
	svc := task.NewTaskService(repo, log, newCLIProgress(log), config.ResolveDefaultTaskStatus(cfg, globalCfg))

	listed, err := svc.ListTasks(cfg.ProjectSlug, filter, page.number, size)
	if err != nil {
		return err
	}

	printTaskList(log, listed)
	return nil
}

// loadMutedColor returns what colors text in the muted role of the loaded
// theme. A theme that fails to load is logged and leaves the text plain.
func loadMutedColor(log *common.Logger) func(text string) string {
	palette, err := theme.Load("")
	if err != nil {
		log.Error("cannot color the context rows: %v", err)
		return func(text string) string { return text }
	}
	return func(text string) string { return palette.Color(theme.RoleMuted) + text + palette.Reset() }
}

// printTaskList prints a page of a listing and its footer, indenting the tasks
// listed under a parent and muting the context rows. The blockers column is as
// wide as its widest value on the page.
func printTaskList(log *common.Logger, listed common.Page[task.ListedTask]) {
	if listed.TotalItems == 0 {
		log.Info("No tasks found")
		return
	}

	blockedByWidth := len(blockedByTitle)
	rows := make([][]string, 0, len(listed.Items))
	rowColors := make([]func(text string) string, len(listed.Items))
	for index, entry := range listed.Items {
		if entry.IsContext {
			rowColors[index] = loadMutedColor(log)
		}
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
		{Title: "STATUS", Width: taskStatusWidth, Color: loadStatusColor(log)},
		{Title: "ID", Width: task.ShortIDLength},
		{Title: "TITLE", Width: taskTitleWidth},
		{Title: blockedByTitle, Width: blockedByWidth},
		{Title: "TICKET"},
	}
	printColoredList(log, "Tasks", listed.TotalItems, columns, rows, rowColors)
	printPageFooter(log, listed.Number, listed.TotalPages)
}
