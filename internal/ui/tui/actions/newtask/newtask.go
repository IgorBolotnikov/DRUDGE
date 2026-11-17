// Package newtask is the command that adds a task written in the editor.
package newtask

import (
	"errors"
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/IgorBolotnikov/DRUDGE/internal/task"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/actions"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/views"
)

const (
	verdictFormat      = "%s  %s added"
	nothingChangedText = "nothing changed"
	draftKeptFormat    = "%w · n to reopen your draft"
	editorFailedFormat = "editor %s failed: %w"
)

// Kind opens the draft of a new task in the editor and adds the task the
// draft describes the way drg task new does. Its verdict names the new task.
// A draft that describes no task skips, and the error of a draft that stays
// points at it.
var Kind = actions.Kind{Name: views.NewTaskCommand, Run: run}

func run(services actions.Services, projectSlug string, command actions.Command, msg views.CommandMsg) tea.Msg {
	path, err := services.Tasks.OpenDraft(projectSlug)
	if err != nil {
		return actions.DoneMsg{Command: command, Err: err}
	}
	return services.Editor.Edit(path, func(err error) tea.Msg {
		if err != nil {
			return actions.DoneMsg{Command: command, Err: fmt.Errorf(editorFailedFormat, services.Editor.Name(), err)}
		}
		// The program calls this callback on its UI loop. A batch runs the
		// create off the loop, because the create reports its events to the
		// program and blocks until the loop takes them.
		return tea.BatchMsg{func() tea.Msg { return create(services, projectSlug, command) }}
	})()
}

func create(services actions.Services, projectSlug string, command actions.Command) actions.DoneMsg {
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
