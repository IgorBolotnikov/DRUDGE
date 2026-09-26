package cmd

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/IgorBolotnikov/DRUDGE/internal/adapters/persistence"
	"github.com/IgorBolotnikov/DRUDGE/internal/common"
	"github.com/IgorBolotnikov/DRUDGE/internal/config"
	"github.com/IgorBolotnikov/DRUDGE/internal/task"
)

var TaskCmd = &Cmd{
	Name: "task",
	Desc: "Task management commands",
	Subcommands: []*Cmd{
		{Name: newSubcommand, Desc: "Add a task to the project", Run: taskNew},
		{Name: listSubcommand, Desc: "List the tasks of the project", Run: taskList},
		{
			Name: "next",
			Desc: "Print the oldest todo task that is ready to run",
			Help: "Print the oldest todo task whose blockers are all done. It starts nothing.\n" +
				"Work of a blocker that is not merged yet is listed under the task.",
			Setup: func(*flag.FlagSet) func(args []string) error { return taskNext },
		},
		{
			Name: "show",
			Args: []string{taskIDArg},
			Desc: "Print one task in full",
			Help: "Print one task in full: its description, where it stands and what its last run left behind.\n" +
				"The task ID may be the short one a listing prints, as long as it names a single task.",
			Setup: func(*flag.FlagSet) func(args []string) error { return taskShow },
		},
		{Name: editSubcommand, Desc: "Change the fields of a task", Run: taskEdit},
		{
			Name: "rm",
			Args: []string{taskIDArg},
			Desc: "Delete a task",
			Help: "Delete a task and the run directory of its Sessions. It asks first.\n" +
				"The branches of the task that hold no commits go with it, and the ones holding work stay.\n" +
				"A task an agent is still working on is refused, so kill its Drudger before removing it.\n" +
				"Tasks blocked by it are unblocked and tasks belonging to it are ungrouped. They stay where they are.\n" +
				"The task ID may be the short one a listing prints, as long as it names a single task.",
			Setup: func(fs *flag.FlagSet) func(args []string) error {
				isForced := fs.Bool(forceFlagName, false, "Remove the task without asking")
				alias(fs, forceFlagShortName, forceFlagName)
				return func(args []string) error { return taskRemove(task.TaskID(args[0]), *isForced) }
			},
		},
		{
			Name: "run",
			Args: []string{taskIDArg},
			Desc: "Hand a task in todo status to a coding agent",
			Help: "Hand a task in todo status to a coding agent.",
			Setup: func(fs *flag.FlagSet) func(args []string) error {
				isDryRun := fs.Bool(dryRunFlagName, false, dryRunUsage)
				return func(args []string) error { return taskRun(task.TaskID(args[0]), *isDryRun) }
			},
		},
		{
			Name: "rerun",
			Args: []string{taskIDArg},
			Desc: "Start a task over from scratch",
			Help: "Start a task over from scratch, clearing what its last run left behind.\n" +
				"Only a task an agent has already had can be rerun, so in-progress and fucked-up.",
			Setup: func(fs *flag.FlagSet) func(args []string) error {
				isDryRun := fs.Bool(dryRunFlagName, false, dryRunUsage)
				return func(args []string) error { return taskRerun(task.TaskID(args[0]), *isDryRun) }
			},
		},
		{
			Name:  "status",
			Args:  []string{taskIDArg},
			Desc:  "Tell how the agent working on a task is doing",
			Help:  "Tell whether the agent working on a task is working, stuck or has .",
			Setup: func(*flag.FlagSet) func(args []string) error { return taskSessionStatus },
		},
	},
}

const taskIDArg = "task-id"

const (
	dryRunFlagName = "dry-run"
	dryRunUsage    = "Print the prompt the agent would get and stop"
)

// CLI flag names.
const (
	helpFlag       = "--help"
	helpFlagShort  = "-h"
	forceFlag      = "--force"
	forceFlagShort = "-f"
)

// Flags a task command reads a value after.
const (
	titleFlag           = "--title"
	descriptionFlag     = "--description"
	descriptionFileFlag = "--description-file"
	ticketFlag          = "--ticket"
	statusFlag          = "--status"
	blockedByFlag       = "--blocked-by"
	blockFlag           = "--block"
	unblockFlag         = "--unblock"
	parentFlag          = "--parent"
)

// stdinPath is the description file path that reads stdin.
const stdinPath = "-"

var (
	errDescriptionFileNeedsPath = errors.New(descriptionFileFlag + " needs a path, or " + stdinPath + " to read stdin")
	errTwoDescriptions          = errors.New(descriptionFlag + " and " + descriptionFileFlag + " cannot be used together")
)

// taskOptionLine lays out one option of the new and edit help, wide enough
// for the longest flag they list.
const taskOptionLine = "  %-27s %s\n"

// taskIDListSeparator splits a flag value holding several task ids.
const taskIDListSeparator = ","

const (
	taskNewUsage = "usage: drg task new " + titleFlag + " <title> (" + descriptionFlag + " <text> | " + descriptionFileFlag + " <path>) [" +
		ticketFlag + " <ticket>] [" + statusFlag + " <status>] [" + blockedByFlag + " <id>[,<id>...]] [" + parentFlag + " <id>]"
	taskListUsage = "usage: drg task list [" + statusFlag + " <status>] [" + ticketFlag + " <ticket>] [" + parentFlag + " <id>]"
	taskEditUsage = "usage: drg task edit <task-id> [" + titleFlag + " <title>] [" + descriptionFlag + " <text>] [" + descriptionFileFlag + " <path>] [" +
		ticketFlag + " <ticket>] [" + statusFlag + " <status>] [" + blockedByFlag + " <id>[,<id>...]] [" + parentFlag + " <id>] [" + forceFlag + "]"
)

// Names of the task subcommands that parse their own args.
const (
	newSubcommand  = "new"
	listSubcommand = "list"
	editSubcommand = "edit"
)

// taskRerunCommand is what a user types to start a task over. Other commands
// name it when starting a task over is the next step.
const taskRerunCommand = "drg task rerun"

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

// readDescriptionFile reads a description from the file at path, or from stdin
// when path is stdinPath. It trims the one trailing newline a heredoc adds and
// refuses content that is blank.
func readDescriptionFile(path string, stdin io.Reader) (string, error) {
	var content []byte
	var err error
	source := "stdin"
	if path == stdinPath {
		if content, err = io.ReadAll(stdin); err != nil {
			return "", fmt.Errorf("cannot read the description from stdin: %w", err)
		}
	} else {
		source = fmt.Sprintf("description file %q", path)
		if content, err = os.ReadFile(path); err != nil {
			var pathErr *fs.PathError
			if errors.As(err, &pathErr) {
				err = pathErr.Err
			}
			return "", fmt.Errorf("cannot read the %s: %w", source, err)
		}
	}

	description := strings.TrimSuffix(string(content), "\n")
	if strings.TrimSpace(description) == "" {
		return "", fmt.Errorf("%s holds no description", source)
	}
	return description, nil
}

func taskNew(args []string) error {
	if hasFlag(args, helpFlag) || hasFlag(args, helpFlagShort) {
		fmt.Println(taskNewUsage)
		fmt.Println()
		fmt.Println("Add a task to the project linked to the current directory.")
		fmt.Println()
		fmt.Println("Options:")
		fmt.Printf(taskOptionLine, titleFlag+" <title>", "Title of the task")
		fmt.Printf(taskOptionLine, descriptionFlag+" <text>", "Description, the prompt the agent is handed")
		fmt.Printf(taskOptionLine, descriptionFileFlag+" <path>", "File to read the description from, "+stdinPath+" to read stdin")
		fmt.Printf(taskOptionLine, ticketFlag+" <ticket>", "Ticket the task came from")
		fmt.Printf(taskOptionLine, statusFlag+" <status>", "Status ("+task.FormatStatuses(task.Statuses)+"), "+config.DefaultTaskStatusKey+" from the config when left out, "+string(task.StatusDraft)+" when that is unset")
		fmt.Printf(taskOptionLine, blockedByFlag+" <id>[,<id>...]", "Tasks this task waits for")
		fmt.Printf(taskOptionLine, parentFlag+" <id>", "Task this task belongs to")
		return nil
	}

	dto, err := parseTaskNewArgs(args, os.Stdin)
	if err != nil {
		return err
	}

	cfg, err := config.LoadLocal()
	if err != nil {
		return err
	}
	globalCfg, err := config.Load()
	if err != nil {
		return err
	}

	log := common.NewLogger("")
	repo := persistence.NewFileTaskRepository(cfg.ProjectSlug)
	svc := task.NewTaskService(repo, log)

	dto.ProjectSlug = cfg.ProjectSlug
	dto.DefaultStatus = config.ResolveDefaultTaskStatus(cfg, globalCfg)
	dto.CreatedAt = time.Now().UTC()

	_, err = svc.CreateTask(dto)
	return err
}

// parseTaskNewArgs reads the fields of a new task. It leaves the project slug
// and the creation time for the caller to fill in.
func parseTaskNewArgs(args []string, stdin io.Reader) (task.CreateTaskDto, error) {
	for _, flag := range []string{blockFlag, unblockFlag} {
		if hasFlag(args, flag) {
			return task.CreateTaskDto{}, fmt.Errorf("drg task new takes no %s, name the blockers of a new task with %s", flag, blockedByFlag)
		}
	}

	title, hasTitle := parseFlagValue(args, titleFlag)
	if !hasTitle || title == "" {
		return task.CreateTaskDto{}, fmt.Errorf("%s is required", titleFlag)
	}

	description, hasDesc := parseFlagValue(args, descriptionFlag)
	descriptionPath, hasDescPath := parseFlagValue(args, descriptionFileFlag)
	switch {
	case hasFlag(args, descriptionFileFlag) && !hasDescPath:
		return task.CreateTaskDto{}, errDescriptionFileNeedsPath
	case hasDesc && hasDescPath:
		return task.CreateTaskDto{}, errTwoDescriptions
	case hasDescPath:
		var err error
		if description, err = readDescriptionFile(descriptionPath, stdin); err != nil {
			return task.CreateTaskDto{}, err
		}
	case !hasDesc:
		return task.CreateTaskDto{}, fmt.Errorf("%s or %s is required", descriptionFlag, descriptionFileFlag)
	}

	ticketID, _ := parseFlagValue(args, ticketFlag)
	blockedBy, _ := parseFlagValue(args, blockedByFlag)
	parentID, _ := parseFlagValue(args, parentFlag)
	statusValue, hasStatus := parseFlagValue(args, statusFlag)
	var status task.TaskStatus
	if hasStatus {
		status = task.TaskStatus(statusValue)
		if !task.KnownStatus(status) {
			return task.CreateTaskDto{}, invalidStatusError(status)
		}
	}

	return task.CreateTaskDto{
		Title:        title,
		Description:  description,
		Status:       status,
		TicketID:     ticketID,
		BlockedBy:    parseTaskIDList(blockedBy),
		ParentTaskID: task.TaskID(parentID),
	}, nil
}

// taskShow prints everything drudge knows about one task.
func taskShow(args []string) error {
	taskID := task.TaskID(args[0])

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

	unmerged, err := deps.drudger.UnmergedWork(blockers)
	if err != nil {
		return err
	}

	family, err := deps.tasks.Family(deps.localCfg.ProjectSlug, found)
	if err != nil {
		return err
	}

	printTask(deps.log, found, blockers, unmerged, family, time.Now().UTC())
	return nil
}

func taskRun(taskID task.TaskID, isDryRun bool) error {
	deps, err := newCommandDeps()
	if err != nil {
		return err
	}

	return deps.drudger.RunTask(deps.localCfg.ProjectSlug, taskID, isDryRun)
}

// taskRerun hands a task back to a Drudger and starts it over.
func taskRerun(taskID task.TaskID, isDryRun bool) error {
	deps, err := newCommandDeps()
	if err != nil {
		return err
	}

	return deps.drudger.RerunTask(deps.localCfg.ProjectSlug, taskID, isDryRun)
}

// taskSessionStatus reports how the last Session of a task is going.
func taskSessionStatus(args []string) error {
	taskID := task.TaskID(args[0])

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

// invalidStatusError names a status drudge does not understand, and lists the
// ones it does.
func invalidStatusError(status task.TaskStatus) error {
	return fmt.Errorf("invalid status %q, must be one of: %s", status, task.FormatStatuses(task.Statuses))
}
