// Package nuke is the command that deletes a Drudger.
package nuke

import (
	"errors"
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/IgorBolotnikov/DRUDGE/internal/drudger"
	"github.com/IgorBolotnikov/DRUDGE/internal/task"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/actions"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/views"
)

const (
	questionFormat       = "Nuke %s? Its sandbox is deleted."
	forcedQuestionFormat = "Nuke %s while it works on %s? Its agent is killed and the task is fucked up."
	verdictFormat        = "%s nuked"
	busyHintFormat       = "busy with %s, X to force"
)

// Kind deletes a Drudger the way drg drudger nuke does once the user confirms
// it. A forced nuke of a busy Drudger asks with the task it kills. The hint of
// a refused nuke names the task the Drudger is busy with.
var Kind = actions.Kind{Name: views.NukeCommand, Run: run, Question: question}

func question(command actions.Command, msg views.CommandMsg) string {
	nukeMsg := msg.(views.NukeMsg)
	if nukeMsg.IsForced && nukeMsg.TaskID != "" {
		return fmt.Sprintf(forcedQuestionFormat, command.Target, task.ShortID(nukeMsg.TaskID))
	}
	return fmt.Sprintf(questionFormat, command.Target)
}

func run(services actions.Services, projectSlug string, command actions.Command, msg views.CommandMsg) tea.Msg {
	nukeMsg := msg.(views.NukeMsg)
	done := actions.DoneMsg{Command: command, Feeds: []views.FeedName{views.DrudgersFeed, views.TasksFeed}}
	if err := services.Drudgers.NukeDrudger(projectSlug, nukeMsg.Slot, nukeMsg.IsForced); err != nil {
		done.Err = err
		var busy *drudger.DrudgerBusyError
		if errors.As(err, &busy) {
			done.Hint = fmt.Sprintf(busyHintFormat, task.ShortID(busy.TaskID))
		}
		return done
	}
	done.Verdict = fmt.Sprintf(verdictFormat, command.Target)
	return done
}
