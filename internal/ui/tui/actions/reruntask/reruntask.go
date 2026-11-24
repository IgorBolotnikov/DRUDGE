// Package reruntask is the command that starts a task over.
package reruntask

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/actions"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/actions/runtask"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/views"
)

const questionFormat = "Start %s over? What its last run left behind is cleared."

// Kind starts a task over the way drg task rerun does once the user confirms
// it. Its verdict names the Drudger that took the task, like a run.
var Kind = actions.Kind{Name: views.RerunTaskCommand, Run: run, Question: question}

func question(command actions.Command) string {
	return fmt.Sprintf(questionFormat, command.Target)
}

func run(services actions.Services, projectSlug string, command actions.Command, msg views.CommandMsg) tea.Msg {
	taskID := msg.(views.RerunTaskMsg).TaskID
	done := actions.DoneMsg{Command: command, Feeds: []views.FeedName{views.TasksFeed, views.DrudgersFeed}}
	if err := services.Drudgers.RerunTask(projectSlug, taskID, false); err != nil {
		done.Err = err
		return done
	}
	done.Verdict = runtask.Verdict(services, projectSlug, command, taskID)
	return done
}
