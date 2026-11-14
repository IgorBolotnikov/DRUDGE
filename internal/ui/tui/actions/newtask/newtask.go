// Package newtask is the command that adds a task written in the editor.
package newtask

import (
	"fmt"

	"github.com/IgorBolotnikov/DRUDGE/internal/task"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/actions"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/views"
)

const verdictFormat = "%s  %s added"

// Kind adds the task of a draft the way drg task new does. Its verdict names
// the new task.
var Kind = actions.Kind{Name: views.NewTaskCommand, Run: run}

func run(services actions.Services, projectSlug string, command actions.Command, msg views.CommandMsg) actions.DoneMsg {
	draft := msg.(views.NewTaskMsg).Draft
	done := actions.DoneMsg{Command: command, Feeds: []views.FeedName{views.TasksFeed}}
	created, err := services.Tasks.CreateTask(draft.Dto(projectSlug))
	if err != nil {
		done.Err = err
		return done
	}
	done.Verdict = fmt.Sprintf(verdictFormat, task.ShortID(created.ID), created.Title)
	return done
}
