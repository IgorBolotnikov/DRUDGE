// Package removetask is the command that deletes a task.
package removetask

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/IgorBolotnikov/DRUDGE/internal/task"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/report"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/actions"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/views"
)

const (
	removedVerdictFormat = "%s removed"
	keptVerdictFormat    = "%s kept"
)

// Kind deletes a task the way drg task rm does. It asks with the removal
// summary and blocks until the user answers. A no leaves the task alone.
var Kind = actions.Kind{Name: views.RemoveTaskCommand, Run: run}

func run(services actions.Services, projectSlug string, command actions.Command, msg views.CommandMsg) tea.Msg {
	taskID := msg.(views.RemoveTaskMsg).TaskID
	done := actions.DoneMsg{Command: command, Feeds: []views.FeedName{views.TasksFeed}}
	isDeclined := false
	confirm := func(removal task.Removal) (bool, error) {
		isYes := services.Asker.Confirm(report.Removal(removal))
		isDeclined = !isYes
		return isYes, nil
	}
	if err := services.Tasks.RemoveTask(projectSlug, taskID, false, services.Sessions, confirm); err != nil {
		done.Err = err
		return done
	}
	if isDeclined {
		done.IsSkipped = true
		done.Verdict = fmt.Sprintf(keptVerdictFormat, command.Target)
		return done
	}
	done.Verdict = fmt.Sprintf(removedVerdictFormat, command.Target)
	return done
}
