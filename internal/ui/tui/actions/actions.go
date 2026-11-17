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

// DrudgerService runs a task the way drg task run does, marks a task done the
// way drg task done does and reads the Drudgers of a project.
type DrudgerService interface {
	RunTask(projectSlug string, requestedID task.TaskID, isDryRun bool) error
	MarkDone(projectSlug string, id task.TaskID) (*task.Task, []*task.Task, error)
	ReadDrudgers(projectSlug string) ([]*drudger.Drudger, error)
}

// TaskService opens the draft of a new task and adds the task it describes.
type TaskService interface {
	OpenDraft(projectSlug string) (string, error)
	CreateTaskFromDraft(projectSlug string) (*task.Task, error)
}

// Services are the services commands run.
type Services struct {
	Drudgers DrudgerService
	Tasks    TaskService
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
// gets the command message of its own kind.
type RunFunc func(services Services, projectSlug string, command Command, msg views.CommandMsg) DoneMsg

// Kind describes one kind of command.
type Kind struct {
	Name views.CommandName
	Run  RunFunc
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
	kind, ok := actions.findKind(msg.CommandName())
	if !ok {
		panic(fmt.Sprintf("a view asked for command %q, which no command kind runs", msg.CommandName()))
	}
	command := Command{Name: msg.CommandName(), Target: msg.Target()}
	return command, func() tea.Msg { return kind.Run(actions.services, actions.projectSlug, command, msg) }
}

func (actions *Actions) findKind(name views.CommandName) (Kind, bool) {
	for _, kind := range actions.kinds {
		if kind.Name == name {
			return kind, true
		}
	}
	return Kind{}, false
}

// OpenDraft opens the draft of a new task of the project and returns its path.
func (actions *Actions) OpenDraft() (string, error) {
	return actions.services.Tasks.OpenDraft(actions.projectSlug)
}
