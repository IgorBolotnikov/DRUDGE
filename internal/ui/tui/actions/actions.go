// Package actions runs the commands the Views of the TUI ask for. It holds the
// services, runs the service method of each command message off the UI loop
// and returns its outcome as a message. The services decide whether a command
// is allowed.
package actions

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/IgorBolotnikov/DRUDGE/internal/drudger"
	"github.com/IgorBolotnikov/DRUDGE/internal/task"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/views"
)

// RunCommand names the command that hands a task to a coding agent.
const RunCommand = "run"

const (
	runVerdictFormat          = "%s handed to Drudger %d"
	runVerdictNoDrudgerFormat = "%s handed to a Drudger"
)

// DrudgerService runs a task the way drg task run does and reads the Drudgers
// of a project.
type DrudgerService interface {
	RunTask(projectSlug string, requestedID task.TaskID, isDryRun bool) error
	ReadDrudgers(projectSlug string) ([]*drudger.Drudger, error)
}

// Services are the services commands run.
type Services struct {
	Drudgers DrudgerService
}

// Command names a command and what it works on, like run 3f9a1c2e.
type Command struct {
	Name   string
	Target string
}

func (command Command) String() string {
	return command.Name + " " + command.Target
}

// DoneMsg carries the end of a command: the verdict of a command that worked
// or the error of one that failed, and the feeds whose data the command may
// have changed.
type DoneMsg struct {
	Command Command
	Verdict string
	Err     error
	Feeds   []views.FeedName
}

// Actions runs the commands of the Views on the services of one project.
type Actions struct {
	services    Services
	projectSlug string
}

// New returns the Actions of a project.
func New(services Services, projectSlug string) *Actions {
	return &Actions{services: services, projectSlug: projectSlug}
}

// Start returns the command a message asks for and the command that runs it.
// It panics on a command message it does not know, which is a bug in the
// View that sent it.
func (actions *Actions) Start(msg views.CommandMsg) (Command, tea.Cmd) {
	switch msg := msg.(type) {
	case views.RunTaskMsg:
		command := Command{Name: RunCommand, Target: task.ShortID(msg.TaskID)}
		return command, func() tea.Msg { return actions.runTask(command, msg.TaskID) }
	}
	panic(fmt.Sprintf("a view asked for command %T, which the TUI does not run", msg))
}

// runTask runs a task. Its verdict names the Drudger that took the task.
func (actions *Actions) runTask(command Command, taskID task.TaskID) DoneMsg {
	done := DoneMsg{Command: command, Feeds: []views.FeedName{views.TasksFeed, views.DrudgersFeed}}
	if err := actions.services.Drudgers.RunTask(actions.projectSlug, taskID, false); err != nil {
		done.Err = err
		return done
	}
	done.Verdict = actions.runVerdict(command, taskID)
	return done
}

// runVerdict names the Drudger that holds a task. The run already worked, so
// a failed read of the Drudgers leaves the Drudger out of the verdict.
func (actions *Actions) runVerdict(command Command, taskID task.TaskID) string {
	drudgers, err := actions.services.Drudgers.ReadDrudgers(actions.projectSlug)
	if err != nil {
		return fmt.Sprintf(runVerdictNoDrudgerFormat, command.Target)
	}
	for _, holder := range drudgers {
		if holder.TaskID == taskID {
			return fmt.Sprintf(runVerdictFormat, command.Target, holder.Slot)
		}
	}
	return fmt.Sprintf(runVerdictNoDrudgerFormat, command.Target)
}
