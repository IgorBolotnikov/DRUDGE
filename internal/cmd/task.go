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

// Flags a task command reads a value after.
const (
	titleFlag       = "--title"
	descriptionFlag = "--description"
	ticketFlag      = "--ticket"
	statusFlag      = "--status"
	blockedByFlag   = "--blocked-by"
)

// taskIDListSeparator splits a flag value holding several task ids.
const taskIDListSeparator = ","

const (
	taskUsage     = "usage: drg task <new|list|show|edit|rm|run|rerun|status>"
	taskListUsage = "usage: drg task list [" + statusFlag + " <status>] [" + ticketFlag + " <ticket>]"
	taskShowUsage = "usage: drg task show <task-id>"
	taskEditUsage = "usage: drg task edit <task-id> [" + titleFlag + " <title>] [" + descriptionFlag + " <text>] [" +
		ticketFlag + " <ticket>] [" + statusFlag + " <status>] [" + blockedByFlag + " <id>[,<id>...]] [" + forceFlag + "]"
	taskRmUsage     = "usage: drg task rm <task-id> [" + forceFlag + "]"
	taskRunUsage    = "usage: drg task run <task-id> [" + dryRunFlag + "]"
	taskRerunUsage  = "usage: drg task rerun <task-id> [" + dryRunFlag + "]"
	taskStatusUsage = "usage: drg task status <task-id>"
)

// Names of the subcommands taking a task id, used in the errors their
// argument parsing produces.
const (
	showSubcommand   = "show"
	editSubcommand   = "edit"
	rmSubcommand     = "rm"
	runSubcommand    = "run"
	rerunSubcommand  = "rerun"
	statusSubcommand = "status"
)

// taskRerunCommand is what a user types to start a task over. Other commands
// name it when starting a task over is the next step.
const taskRerunCommand = "drg task rerun"

// taskTitleWidth is how much room a listing gives a task title.
const taskTitleWidth = 40

func runTask(args []string) error {
	if len(args) < 1 {
		return errors.New(taskUsage)
	}

	switch args[0] {
	case "new":
		return taskNew(args[1:])
	case "list":
		return taskList(args[1:])
	case showSubcommand:
		return taskShow(args[1:])
	case editSubcommand:
		return taskEdit(args[1:])
	case rmSubcommand:
		return taskRemove(args[1:])
	case runSubcommand:
		return taskRun(args[1:])
	case rerunSubcommand:
		return taskRerun(args[1:])
	case statusSubcommand:
		return taskSessionStatus(args[1:])
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

// parseTaskIDList reads the comma-separated task ids a flag was given. Blank
// entries are dropped, so an empty value reads as an empty list.
func parseTaskIDList(value string) []task.TaskID {
	ids := []task.TaskID{}
	for _, text := range strings.Split(value, taskIDListSeparator) {
		if text = strings.TrimSpace(text); text != "" {
			ids = append(ids, task.TaskID(text))
		}
	}
	return ids
}

func taskNew(args []string) error {
	title, hasTitle := parseFlagValue(args, titleFlag)
	if !hasTitle || title == "" {
		return fmt.Errorf("%s is required", titleFlag)
	}

	description, hasDesc := parseFlagValue(args, descriptionFlag)
	if !hasDesc {
		return fmt.Errorf("%s is required", descriptionFlag)
	}

	ticketID, _ := parseFlagValue(args, ticketFlag)
	blockedBy, _ := parseFlagValue(args, blockedByFlag)
	statusValue, hasStatus := parseFlagValue(args, statusFlag)
	status := task.StatusDraft
	if hasStatus {
		status = task.TaskStatus(statusValue)
		if !task.KnownStatus(status) {
			return invalidStatusError(status)
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
		Status:      status,
		TicketID:    ticketID,
		ProjectSlug: cfg.ProjectSlug,
		BlockedBy:   parseTaskIDList(blockedBy),
		CreatedAt:   time.Now().UTC(),
	}

	_, err = svc.CreateTask(dto)
	return err
}

func taskList(args []string) error {
	// TODO: make a util for printing out help text
	if hasFlag(args, helpFlag) || hasFlag(args, helpFlagShort) {
		fmt.Println(taskListUsage)
		fmt.Println()
		fmt.Println("List tasks in the current project.")
		fmt.Println()
		fmt.Println("Options:")
		fmt.Printf("  %s <status>  Filter by status (%s)\n", statusFlag, task.FormatStatuses(task.Statuses))
		fmt.Printf("  %s <ticket>  Filter by ticket ID\n", ticketFlag)
		return nil
	}

	var filter task.ListTasksFilter
	if statusValue, hasStatus := parseFlagValue(args, statusFlag); hasStatus {
		status := task.TaskStatus(statusValue)
		if !task.KnownStatus(status) {
			return invalidStatusError(status)
		}
		filter.Status = &status
	}
	if ticketID, hasTicket := parseFlagValue(args, ticketFlag); hasTicket {
		filter.TicketID = &ticketID
	}

	cfg, err := config.LoadLocal()
	if err != nil {
		return err
	}

	log := common.NewLogger("")
	repo := persistence.NewFileTaskRepository(cfg.ProjectSlug)
	svc := task.NewTaskService(repo, log)

	tasks, err := svc.ListTasks(cfg.ProjectSlug, filter)
	if err != nil {
		return fmt.Errorf("could not list tasks: %w", err)
	}

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
		rows = append(rows, []string{string(t.Status), task.ShortID(t.ID), t.Title, t.TicketID})
	}

	printList(log, "Tasks", columns, rows)
	return nil
}

// taskShow prints everything drudge knows about one task.
func taskShow(args []string) error {
	if hasFlag(args, helpFlag) || hasFlag(args, helpFlagShort) {
		fmt.Println(taskShowUsage)
		fmt.Println()
		fmt.Println("Print one task in full: its description, where it stands and what its last run left behind.")
		fmt.Println("The task ID may be the short one a listing prints, as long as it names a single task.")
		return nil
	}

	taskID, err := parseTaskIDArgs(args, showSubcommand, taskShowUsage)
	if err != nil {
		return err
	}

	deps, err := newCommandDeps()
	if err != nil {
		return err
	}

	found, err := deps.tasks.GetTask(deps.localCfg.ProjectSlug, taskID)
	if err != nil {
		return err
	}

	blockers, err := deps.tasks.Blockers(deps.localCfg.ProjectSlug, found)
	if err != nil {
		return err
	}

	printTask(deps.log, found, blockers, time.Now().UTC())
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

	taskID, isDryRun, err := parseTaskRunArgs(args, runSubcommand, taskRunUsage)
	if err != nil {
		return err
	}

	deps, err := newCommandDeps()
	if err != nil {
		return err
	}

	return deps.drudger.RunTask(deps.localCfg.ProjectSlug, taskID, isDryRun)
}

// taskRerun hands a task back to a Drudger and starts it over.
func taskRerun(args []string) error {
	if hasFlag(args, helpFlag) || hasFlag(args, helpFlagShort) {
		fmt.Println(taskRerunUsage)
		fmt.Println()
		fmt.Println("Start a task over from scratch, clearing what its last run left behind.")
		fmt.Println("Only a task an agent has already had can be rerun, so in-progress and fucked-up.")
		fmt.Println()
		fmt.Println("Options:")
		fmt.Printf("  %s  Print the prompt the agent would get and stop\n", dryRunFlag)
		return nil
	}

	taskID, isDryRun, err := parseTaskRunArgs(args, rerunSubcommand, taskRerunUsage)
	if err != nil {
		return err
	}

	deps, err := newCommandDeps()
	if err != nil {
		return err
	}

	return deps.drudger.RerunTask(deps.localCfg.ProjectSlug, taskID, isDryRun)
}

// taskSessionStatus reports how the last Session of a task is going.
func taskSessionStatus(args []string) error {
	if hasFlag(args, helpFlag) || hasFlag(args, helpFlagShort) {
		fmt.Println(taskStatusUsage)
		fmt.Println()
		fmt.Println("Tell whether the agent working on a task is working, stuck or has .")
		return nil
	}

	taskID, err := parseTaskIDArgs(args, statusSubcommand, taskStatusUsage)
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

// parseTaskIDArgs reads the single task id a subcommand takes. The subcommand
// names itself, so its errors say which one the user typed. The id reaches the
// lookup as typed, which is what lets a short id from a listing resolve.
func parseTaskIDArgs(args []string, subcommand, usage string) (task.TaskID, error) {
	var taskID string

	for _, arg := range args {
		switch {
		case strings.HasPrefix(arg, "-"):
			return "", fmt.Errorf("unknown flag %q, %s", arg, usage)
		case taskID == "":
			taskID = arg
		default:
			return "", fmt.Errorf("unexpected argument %q, drg task %s takes a single task ID", arg, subcommand)
		}
	}

	if taskID == "" {
		return "", fmt.Errorf("task ID is required, %s", usage)
	}

	return task.TaskID(taskID), nil
}

// parseTaskRunArgs reads the task id and the dry run flag that both launch
// subcommands take. The subcommand names itself, so its errors say which one
// the user typed.
func parseTaskRunArgs(args []string, subcommand, usage string) (task.TaskID, bool, error) {
	var taskID string
	isDryRun := false

	for _, arg := range args {
		switch {
		case arg == dryRunFlag:
			isDryRun = true
		case strings.HasPrefix(arg, "-"):
			return "", false, fmt.Errorf("unknown flag %q, %s", arg, usage)
		case taskID == "":
			taskID = arg
		default:
			return "", false, fmt.Errorf("unexpected argument %q, drg task %s takes a single task ID", arg, subcommand)
		}
	}

	if taskID == "" {
		return "", false, fmt.Errorf("task ID is required, %s", usage)
	}

	return task.TaskID(taskID), isDryRun, nil
}

// invalidStatusError names a status drudge does not understand, and lists the
// ones it does.
func invalidStatusError(status task.TaskStatus) error {
	return fmt.Errorf("invalid status %q, must be one of: %s", status, task.FormatStatuses(task.Statuses))
}
