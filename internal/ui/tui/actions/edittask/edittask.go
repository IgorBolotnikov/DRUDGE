// Package edittask is the command that changes a task to what its draft in
// the editor describes.
package edittask

import (
	"errors"
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/IgorBolotnikov/DRUDGE/internal/task"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/actions"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/views"
)

const (
	verdictFormat      = "%s  %s edited · %s"
	fieldSeparator     = ", "
	nothingChangedText = "nothing changed"
	draftKeptFormat    = "%w · e to reopen your draft"
	editorFailedFormat = "editor %s failed: %w"
)

// Kind opens the draft of a task in the editor and changes the task to what
// the draft describes the way drg task edit does. Its verdict names the task
// and the fields that changed. A draft that changes nothing skips, and the
// error of a draft that stays points at it.
var Kind = actions.Kind{Name: views.EditTaskCommand, Run: run}

func run(services actions.Services, projectSlug string, command actions.Command, msg views.CommandMsg) tea.Msg {
	taskID := msg.(views.EditTaskMsg).TaskID
	path, err := services.Tasks.OpenTaskDraft(projectSlug, taskID)
	if err != nil {
		return actions.DoneMsg{Command: command, Err: err}
	}
	return services.Editor.Edit(path, func(err error) tea.Msg {
		if err != nil {
			return actions.DoneMsg{Command: command, Err: fmt.Errorf(editorFailedFormat, services.Editor.Name(), err)}
		}
		// The program calls this callback on its UI loop. A batch runs the
		// edit off the loop, because the edit reports its events to the
		// program and blocks until the loop takes them.
		return tea.BatchMsg{func() tea.Msg { return edit(services, projectSlug, command, taskID) }}
	})()
}

func edit(services actions.Services, projectSlug string, command actions.Command, taskID task.TaskID) actions.DoneMsg {
	done := actions.DoneMsg{Command: command, Feeds: []views.FeedName{views.TasksFeed}}
	edited, fields, err := services.Drudgers.EditTaskFromDraft(projectSlug, taskID)
	switch {
	case errors.Is(err, task.ErrDraftUnchanged):
		done.IsSkipped = true
		done.Verdict = nothingChangedText
	case err != nil:
		done.Err = fmt.Errorf(draftKeptFormat, err)
	default:
		done.Verdict = fmt.Sprintf(verdictFormat, task.ShortID(edited.ID), edited.Title, strings.Join(fields, fieldSeparator))
	}
	return done
}
