package cmd

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"drudge/internal/adapters/persistence"
	"drudge/internal/common"
	"drudge/internal/config"
	"drudge/internal/task"
)

var TaskCmd = &Cmd{
	Name:  "task",
	Usage: "task <subcommand>",
	Desc:  "Task management commands",
	Run:   runTask,
}

// CLI flag names.
const (
	dryRunFlag     = "--dry-run"
	helpFlag       = "--help"
	helpFlagShort  = "-h"
	forceFlag      = "--force"
	forceFlagShort = "-f"
)

const (
	taskUsage       = "usage: drg task <new|list|run|status>"
	taskRunUsage    = "usage: drg task run <task-id> [" + dryRunFlag + "]"
	taskStatusUsage = "usage: drg task status <task-id>"
)

// taskTitleWidth is how much room a listing gives a task title.
const taskTitleWidth = 40

var validStatuses = []string{
	task.StatusDraft,
	task.StatusTodo,
	task.StatusInProgress,
	task.StatusFuckedUp,
	task.StatusDone,
}

func runTask(args []string) error {
	if len(args) < 1 {
		return errors.New(taskUsage)
	}

	switch args[0] {
	case "new":
		return taskNew(args[1:])
	case "list":
		return taskList(args[1:])
	case "run":
		return taskRun(args[1:])
	case "status":
		return taskStatus(args[1:])
	default:
		return fmt.Errorf("unknown task subcommand %q, %s", args[0], taskUsage)
	}
}

func parseFlagValue(args []string, flag string) (string, bool) {
	for i := range args {
		if args[i] == flag {
			if i+1 >= len(args) {
				return "", false
			}
			return args[i+1], true
		}
	}
	return "", false
}

func hasFlag(args []string, flag string) bool {
	return slices.Contains(args, flag)
}

func taskNew(args []string) error {
	title, hasTitle := parseFlagValue(args, "--title")
	if !hasTitle || title == "" {
		return fmt.Errorf("--title is required")
	}

	description, hasDesc := parseFlagValue(args, "--description")
	if !hasDesc {
		return fmt.Errorf("--description is required")
	}

	ticketID, _ := parseFlagValue(args, "--ticket")
	status, hasStatus := parseFlagValue(args, "--status")
	if !hasStatus {
		status = task.StatusDraft
	} else {
		valid := slices.Contains(validStatuses, status)
		if !valid {
			return fmt.Errorf("invalid status %q, must be one of: %s", status, strings.Join(validStatuses, ", "))
		}
	}

	cfg, err := config.LoadLocal()
	if err != nil {
		return err
	}

	log := common.NewLogger("")
	repo := persistence.NewFileTaskRepository(cfg.ProjectSlug)
	svc := task.NewTaskService(repo, log)

	dto := task.CreateTaskDto{
		Title:       title,
		Description: description,
		Status:      task.TaskStatus(status),
		TicketID:    ticketID,
		ProjectSlug: cfg.ProjectSlug,
		CreatedAt:   time.Now().UTC(),
	}

	_, err = svc.CreateTask(dto)
	return err
}

func taskList(args []string) error {
	// TODO: make a util for printing out help text
	if hasFlag(args, helpFlag) || hasFlag(args, helpFlagShort) {
		fmt.Println("usage: drg task list [--status <status>] [--ticket <ticket>]")
		fmt.Println()
		fmt.Println("List tasks in the current project.")
		fmt.Println()
		fmt.Println("Options:")
		fmt.Println("  --status <status>  Filter by status (draft, todo, in-progress, fucked-up, done)")
		fmt.Println("  --ticket <ticket>  Filter by ticket ID")
		return nil
	}

	statusFilter, hasStatus := parseFlagValue(args, "--status")
	ticketFilter, hasTicket := parseFlagValue(args, "--ticket")

	if hasStatus {
		valid := slices.Contains(validStatuses, statusFilter)
		if !valid {
			return fmt.Errorf("invalid status %q, must be one of: %s", statusFilter, strings.Join(validStatuses, ", "))
		}
	}

	cfg, err := config.LoadLocal()
	if err != nil {
		return err
	}

	log := common.NewLogger("")
	repo := persistence.NewFileTaskRepository(cfg.ProjectSlug)
	svc := task.NewTaskService(repo, log)

	tasks, err := svc.ListTasks(cfg.ProjectSlug)
	if err != nil {
		return fmt.Errorf("could not list tasks: %w", err)
	}

	// TODO: move this to service
	tasks = filterTasks(tasks, statusFilter, hasStatus, ticketFilter, hasTicket)
	tasks = sortTasksDesc(tasks)

	if len(tasks) == 0 {
		log.Info("No tasks found")
		return nil
	}

	columns := []column{
		{Title: "STATUS", Width: 15},
		{Title: "ID", Width: task.ShortIDLength},
		{Title: "TITLE", Width: taskTitleWidth},
		{Title: "TICKET"},
	}
	rows := make([][]string, 0, len(tasks))
	for _, t := range tasks {
		rows = append(rows, []string{string(t.Status), shortTaskID(t.ID), t.Title, t.TicketID})
	}

	printList(log, "Tasks", columns, rows)
	return nil
}

func taskRun(args []string) error {
	if hasFlag(args, helpFlag) || hasFlag(args, helpFlagShort) {
		fmt.Println(taskRunUsage)
		fmt.Println()
		fmt.Println("Hand a task in todo status to a coding agent.")
		fmt.Println()
		fmt.Println("Options:")
		fmt.Printf("  %s  Print the prompt the agent would get and stop\n", dryRunFlag)
		return nil
	}

	taskID, dryRun, err := parseTaskRunArgs(args)
	if err != nil {
		return err
	}

	deps, err := newCommandDeps()
	if err != nil {
		return err
	}

	return deps.drudger.RunTask(deps.localCfg.ProjectSlug, taskID, dryRun)
}

// taskStatus reports how the last Session of a task is going.
func taskStatus(args []string) error {
	if hasFlag(args, helpFlag) || hasFlag(args, helpFlagShort) {
		fmt.Println(taskStatusUsage)
		fmt.Println()
		fmt.Println("Tell whether the agent working on a task is working, stuck or has fallen over.")
		return nil
	}

	taskID, err := parseTaskStatusArgs(args)
	if err != nil {
		return err
	}

	deps, err := newCommandDeps()
	if err != nil {
		return err
	}

	session, err := deps.drudger.SessionStatus(deps.localCfg.ProjectSlug, taskID)
	if err != nil {
		return err
	}

	printSessionStatus(deps.log, session)
	return nil
}

func parseTaskStatusArgs(args []string) (task.TaskID, error) {
	var taskID string

	for _, arg := range args {
		switch {
		case strings.HasPrefix(arg, "-"):
			return "", fmt.Errorf("unknown flag %q, %s", arg, taskStatusUsage)
		case taskID == "":
			taskID = arg
		default:
			return "", fmt.Errorf("unexpected argument %q, drg task status takes a single task ID", arg)
		}
	}

	if taskID == "" {
		return "", fmt.Errorf("task ID is required, %s", taskStatusUsage)
	}

	return task.TaskID(taskID), nil
}

func parseTaskRunArgs(args []string) (task.TaskID, bool, error) {
	var taskID string
	dryRun := false

	for _, arg := range args {
		switch {
		case arg == dryRunFlag:
			dryRun = true
		case strings.HasPrefix(arg, "-"):
			return "", false, fmt.Errorf("unknown flag %q, %s", arg, taskRunUsage)
		case taskID == "":
			taskID = arg
		default:
			return "", false, fmt.Errorf("unexpected argument %q, drg task run takes a single task ID", arg)
		}
	}

	if taskID == "" {
		return "", false, fmt.Errorf("task ID is required, %s", taskRunUsage)
	}

	return task.TaskID(taskID), dryRun, nil
}

func filterTasks(tasks []*task.Task, statusFilter string, hasStatus bool, ticketFilter string, hasTicket bool) []*task.Task {
	var result []*task.Task
	for _, t := range tasks {
		if hasStatus && t.Status != task.TaskStatus(statusFilter) {
			continue
		}
		if hasTicket && t.TicketID != ticketFilter {
			continue
		}
		result = append(result, t)
	}
	return result
}

func sortTasksDesc(tasks []*task.Task) []*task.Task {
	slices.SortFunc(tasks, func(a, b *task.Task) int {
		return b.CreatedAt.Compare(a.CreatedAt)
	})
	return tasks
}
