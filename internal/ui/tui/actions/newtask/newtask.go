// Package newtask is the command that adds a task written in the editor.
package newtask

import (
	"errors"
	"fmt"

	"github.com/IgorBolotnikov/DRUDGE/internal/task"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/actions"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/views"
)

const (
	verdictFormat      = "%s  %s added"
	nothingChangedText = "nothing changed"
	draftKeptFormat    = "%w · n to reopen your draft"
)

// Kind adds the task the draft of a new task describes the way drg task new
// does. Its verdict names the new task. A draft that describes no task skips,
// and the error of a draft that stays points at it.
var Kind = actions.Kind{Name: views.NewTaskCommand, Run: run}

func run(services actions.Services, projectSlug string, command actions.Command, msg views.CommandMsg) actions.DoneMsg {
	done := actions.DoneMsg{Command: command, Feeds: []views.FeedName{views.TasksFeed}}
	created, err := services.Tasks.CreateTaskFromDraft(projectSlug)
	switch {
	case errors.Is(err, task.ErrDraftUnchanged):
		done.IsSkipped = true
		done.Verdict = nothingChangedText
	case err != nil:
		done.Err = fmt.Errorf(draftKeptFormat, err)
	default:
		done.Verdict = fmt.Sprintf(verdictFormat, task.ShortID(created.ID), created.Title)
	}
	return done
}
