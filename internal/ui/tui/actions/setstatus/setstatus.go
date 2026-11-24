// Package setstatus is the command that sets the status of a task.
package setstatus

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/IgorBolotnikov/DRUDGE/internal/task"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/actions"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/views"
)

const (
	questionFormat = "Force %s to %s? drudge maintains this status itself."
	verdictFormat  = "%s now %s"
)

// Kind sets the status of a task the way drg task edit --status does. A forced
// status asks first, and a yes sends the edit with force like --force does.
var Kind = actions.Kind{Name: views.SetStatusCommand, Run: run, Question: question}

func question(command actions.Command, msg views.CommandMsg) (string, bool) {
	statusMsg := msg.(views.SetStatusMsg)
	if !statusMsg.IsForced {
		return "", false
	}
	return fmt.Sprintf(questionFormat, command.Target, statusMsg.Status), true
}

func run(services actions.Services, projectSlug string, command actions.Command, msg views.CommandMsg) tea.Msg {
	statusMsg := msg.(views.SetStatusMsg)
	done := actions.DoneMsg{Command: command, Feeds: []views.FeedName{views.TasksFeed}}
	changes := task.EditTaskDto{Status: &statusMsg.Status, AllowsManagedStatus: statusMsg.IsForced}
	edited, err := services.Drudgers.EditTask(projectSlug, statusMsg.TaskID, changes)
	if err != nil {
		done.Err = err
		return done
	}
	done.Verdict = fmt.Sprintf(verdictFormat, command.Target, edited.Status)
	return done
}
