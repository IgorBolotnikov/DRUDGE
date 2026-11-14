// Package runtask is the command that hands a task to a coding agent.
package runtask

import (
	"fmt"

	"github.com/IgorBolotnikov/DRUDGE/internal/task"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/actions"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/views"
)

const (
	verdictFormat          = "%s handed to Drudger %d"
	verdictNoDrudgerFormat = "%s handed to a Drudger"
)

// Kind runs a task the way drg task run does. Its verdict names the Drudger
// that took the task.
var Kind = actions.Kind{Name: views.RunTaskCommand, Run: run}

func run(services actions.Services, projectSlug string, command actions.Command, msg views.CommandMsg) actions.DoneMsg {
	taskID := msg.(views.RunTaskMsg).TaskID
	done := actions.DoneMsg{Command: command, Feeds: []views.FeedName{views.TasksFeed, views.DrudgersFeed}}
	if err := services.Drudgers.RunTask(projectSlug, taskID, false); err != nil {
		done.Err = err
		return done
	}
	done.Verdict = verdict(services, projectSlug, command, taskID)
	return done
}

// verdict names the Drudger that holds a task. The run already worked, so a
// failed read of the Drudgers leaves the Drudger out of the verdict.
func verdict(services actions.Services, projectSlug string, command actions.Command, taskID task.TaskID) string {
	drudgers, err := services.Drudgers.ReadDrudgers(projectSlug)
	if err != nil {
		return fmt.Sprintf(verdictNoDrudgerFormat, command.Target)
	}
	for _, holder := range drudgers {
		if holder.TaskID == taskID {
			return fmt.Sprintf(verdictFormat, command.Target, holder.Slot)
		}
	}
	return fmt.Sprintf(verdictNoDrudgerFormat, command.Target)
}
