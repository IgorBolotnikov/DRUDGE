package cmd

import (
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

var validStatuses = []string{
	task.StatusDraft,
	task.StatusTodo,
	task.StatusInProgress,
	task.StatusFuckedUp,
	task.StatusDone,
}

func runTask(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: drg task <subcommand>")
	}

	switch args[0] {
	case "new":
		return taskNew(args[1:])
	case "list":
		return taskList(args[1:])
	default:
		return fmt.Errorf("unknown task subcommand: %s", args[0])
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
	if hasFlag(args, "--help") || hasFlag(args, "-h") {
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

	log.Info("Tasks (%d):", len(tasks))
	log.Info("  %-15s  %-8s  %-40s  %s", "STATUS", "ID", "TITLE", "TICKET")
	log.Info("  ---------------  --------  ----------------------------------------  ------")
	maxTitleLen := 40
	for _, t := range tasks {
		id := string(t.ID)
		if len(id) > 8 {
			id = id[:8]
		}
		title := t.Title
		if len(title) > maxTitleLen {
			title = title[:maxTitleLen-3] + "..."
		}
		log.Info("  %-15s  %-8s  %-40s  %s", t.Status, id, title, t.TicketID)
	}

	return nil
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
