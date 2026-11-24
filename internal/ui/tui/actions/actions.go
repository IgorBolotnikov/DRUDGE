// Package actions runs the commands the Views of the TUI ask for. It holds
// what every command shares: the description of a kind of command, the
// services commands run on and the outcome of a command. Each kind of command
// lives in a package of its own below it. The services decide whether a
// command is allowed.
package actions

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/IgorBolotnikov/DRUDGE/internal/drudger"
	"github.com/IgorBolotnikov/DRUDGE/internal/task"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/views"
)

// DrudgerService runs a task the way drg task run does, starts a task over the
// way drg task rerun does, marks a task done the way drg task done does, edits
// a task from its draft and reads the Drudgers of a project.
type DrudgerService interface {
	RunTask(projectSlug string, requestedID task.TaskID, isDryRun bool) error
	RerunTask(projectSlug string, requestedID task.TaskID, isDryRun bool) error
	MarkDone(projectSlug string, id task.TaskID) (*task.Task, []*task.Task, error)
	EditTaskFromDraft(projectSlug string, id task.TaskID) (*task.Task, []string, error)
	ReadDrudgers(projectSlug string) ([]*drudger.Drudger, error)
}

// TaskService opens the draft of a new task and adds the task it describes,
// opens the draft of a task and removes a task the way drg task rm does.
type TaskService interface {
	OpenDraft(projectSlug string) (string, error)
	OpenTaskDraft(projectSlug string, id task.TaskID) (string, error)
	CreateTaskFromDraft(projectSlug string) (*task.Task, error)
	RemoveTask(projectSlug string, id task.TaskID, isForced bool, sessions task.SessionKeeper, confirm task.ConfirmRemoval) error
}

// Editor edits a file in the terminal. The command Edit returns suspends the
// program until the editor exits and then sends the message of done. Name
// names the editor in errors.
type Editor interface {
	Name() string
	Edit(path string, done tea.ExecCallback) tea.Cmd
}

// Asker asks the user a question while a command runs. Confirm blocks until
// the user answers and reports whether the answer is yes. It must be called
// off the UI loop.
type Asker interface {
	Confirm(question string) bool
}

// Services are the services commands run.
type Services struct {
	Drudgers DrudgerService
	Tasks    TaskService
	Sessions task.SessionKeeper
	Editor   Editor
	Asker    Asker
}

// Command names a command and what it works on, like run 3f9a1c2e.
type Command struct {
	Name   views.CommandName
	Target string
}

func (command Command) String() string {
	return string(command.Name) + " " + command.Target
}

// DoneMsg carries the end of a command: the verdict of a command that worked
// or the error of one that failed, and the feeds whose data the command may
// have changed. IsSkipped says the command did nothing, and its verdict says
// why.
type DoneMsg struct {
	Command   Command
	Verdict   string
	IsSkipped bool
	Err       error
	Feeds     []views.FeedName
}

// RunFunc runs a command on the services of a project off the UI loop. It
// gets the command message of its own kind. It returns the DoneMsg of the
// command, or the message of a step that needs the terminal, like an editor.
// The callback of that step leads to the DoneMsg of the command.
type RunFunc func(services Services, projectSlug string, command Command, msg views.CommandMsg) tea.Msg

// QuestionFunc returns the question a command asks before it starts.
type QuestionFunc func(command Command) string

// Kind describes one kind of command. A kind with a question starts only once
// the user answers yes to it.
type Kind struct {
	Name     views.CommandName
	Run      RunFunc
	Question QuestionFunc
}

// Actions runs the commands of the Views on the services of one project.
type Actions struct {
	services    Services
	projectSlug string
	kinds       []Kind
}

// New returns the Actions of a project that run the commands of kinds.
func New(services Services, projectSlug string, kinds []Kind) *Actions {
	return &Actions{services: services, projectSlug: projectSlug, kinds: kinds}
}

// Start returns the command a message asks for and the command that runs it.
// It panics on a command of a kind it does not know, which is a bug in the
// View that sent it.
func (actions *Actions) Start(msg views.CommandMsg) (Command, tea.Cmd) {
	kind := actions.kind(msg)
	command := newCommand(msg)
	return command, func() tea.Msg { return kind.Run(actions.services, actions.projectSlug, command, msg) }
}

// Question returns the question the command a message asks for asks before it
// starts. ok is false for a command that starts right away. It panics on a
// command of a kind it does not know, like Start.
func (actions *Actions) Question(msg views.CommandMsg) (question string, ok bool) {
	kind := actions.kind(msg)
	if kind.Question == nil {
		return "", false
	}
	return kind.Question(newCommand(msg)), true
}

func newCommand(msg views.CommandMsg) Command {
	return Command{Name: msg.CommandName(), Target: msg.Target()}
}

func (actions *Actions) kind(msg views.CommandMsg) Kind {
	for _, kind := range actions.kinds {
		if kind.Name == msg.CommandName() {
			return kind
		}
	}
	panic(fmt.Sprintf("a view asked for command %q, which no command kind runs", msg.CommandName()))
}
