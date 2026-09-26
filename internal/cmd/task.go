package cmd

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
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
		{
			Name: "new",
			Desc: "Add a task to the project",
			Help: "Add a task to the project linked to the current directory.",
			Setup: func(fs *flag.FlagSet) func(args []string) error {
				flags := &taskNewFlags{}
				flags.declare(fs)
				return func([]string) error {
					dto, err := flags.dto(os.Stdin)
					if err != nil {
						return err
					}
					return taskNew(dto)
				}
			},
		},
		{
			Name: "list",
			Desc: "List the tasks of the project",
			Help: "List tasks in the current project. Tasks belonging to another task are listed under it.\n" +
				"A filter lists the tasks it keeps without grouping them.",
			Setup: func(fs *flag.FlagSet) func(args []string) error {
				flags := &taskListFlags{}
				flags.declare(fs)
				return func([]string) error {
					filter, err := flags.filter()
					if err != nil {
						return err
					}
					return taskList(filter)
				}
			},
		},
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
		{
			Name: "edit",
			Args: []string{taskIDArg},
			Desc: "Change the fields of a task",
			Help: "Change what a task asks for and where it stands. It takes at least one field.\n" +
				"The task ID may be the short one a listing prints, as long as it names a single task.",
			Setup: func(fs *flag.FlagSet) func(args []string) error {
				flags := &taskEditFlags{}
				flags.declare(fs)
				return func(args []string) error {
					changes, err := flags.changes(os.Stdin)
					if err != nil {
						return err
					}
					return taskEdit(task.TaskID(args[0]), changes)
				}
			},
		},
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
	helpFlag      = "--help"
	helpFlagShort = "-h"
)

// Flags a task command reads a value after.
const (
	titleFlagName           = "title"
	descriptionFlagName     = "description"
	descriptionFileFlagName = "description-file"
	ticketFlagName          = "ticket"
	statusFlagName          = "status"
	blockedByFlagName       = "blocked-by"
	blockFlagName           = "block"
	unblockFlagName         = "unblock"
	parentFlagName          = "parent"
)

// stdinPath is the description file path that reads stdin.
const stdinPath = "-"

var errTwoDescriptions = errors.New(flagLabel(descriptionFlagName) + " and " + flagLabel(descriptionFileFlagName) + " cannot be used together")

// taskIDListSeparator splits a flag value holding several task ids.
const taskIDListSeparator = ","

// taskRerunCommand is what a user types to start a task over. Other commands
// name it when starting a task over is the next step.
const taskRerunCommand = "drg task rerun"

// optionalOf converts the value of a flag that was given, and returns nil for
// a flag that was not.
func optionalOf[Value ~string](flagValue optionalString) *Value {
	if flagValue.value == nil {
		return nil
	}
	converted := Value(*flagValue.value)
	return &converted
}

// optionalTaskIDList reads the task ids of a flag that was given, and returns
// nil for a flag that was not.
func optionalTaskIDList(flagValue optionalString) *[]task.TaskID {
	if flagValue.value == nil {
		return nil
	}
	ids := parseTaskIDList(*flagValue.value)
	return &ids
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

// taskNewFlags holds the flags of drg task new.
type taskNewFlags struct {
	title           optionalString
	description     optionalString
	descriptionFile optionalString
	ticket          optionalString
	status          optionalString
	blockedBy       optionalString
	parent          optionalString
}

func (flags *taskNewFlags) declare(fs *flag.FlagSet) {
	fs.Var(&flags.title, titleFlagName, "Task `title`")
	fs.Var(&flags.description, descriptionFlagName, "Description, the `text` the agent is handed as its prompt")
	fs.Var(&flags.descriptionFile, descriptionFileFlagName, "Read the description from the file at `path`, "+stdinPath+" to read stdin")
	fs.Var(&flags.ticket, ticketFlagName, "The `ticket` the task came from")
	fs.Var(&flags.status, statusFlagName, "Task `status` ("+task.FormatStatuses(task.Statuses)+"), "+
		config.DefaultTaskStatusKey+" from the config when left out, "+string(task.StatusDraft)+" when that is unset")
	fs.Var(&flags.blockedBy, blockedByFlagName, "Comma-separated `ids` of the tasks this task waits for")
	fs.Var(&flags.parent, parentFlagName, "The `id` of the task this task belongs to")
}

// dto returns the fields of a new task, reading a description file from disk
// or from stdin. It leaves the project slug, the default status and the
// creation time for the caller to fill in.
func (flags *taskNewFlags) dto(stdin io.Reader) (task.CreateTaskDto, error) {
	if flags.title.get() == "" {
		return task.CreateTaskDto{}, fmt.Errorf("%s is required", flagLabel(titleFlagName))
	}

	description := flags.description.get()
	switch {
	case flags.description.value != nil && flags.descriptionFile.value != nil:
		return task.CreateTaskDto{}, errTwoDescriptions
	case flags.descriptionFile.value != nil:
		var err error
		if description, err = readDescriptionFile(flags.descriptionFile.get(), stdin); err != nil {
			return task.CreateTaskDto{}, err
		}
	case flags.description.value == nil:
		return task.CreateTaskDto{}, fmt.Errorf("%s or %s is required", flagLabel(descriptionFlagName), flagLabel(descriptionFileFlagName))
	}

	var status task.TaskStatus
	if flags.status.value != nil {
		status = task.TaskStatus(flags.status.get())
		if !task.KnownStatus(status) {
			return task.CreateTaskDto{}, invalidStatusError(status)
		}
	}

	return task.CreateTaskDto{
		Title:        flags.title.get(),
		Description:  description,
		Status:       status,
		TicketID:     flags.ticket.get(),
		BlockedBy:    parseTaskIDList(flags.blockedBy.get()),
		ParentTaskID: task.TaskID(flags.parent.get()),
	}, nil
}

func taskNew(dto task.CreateTaskDto) error {
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
