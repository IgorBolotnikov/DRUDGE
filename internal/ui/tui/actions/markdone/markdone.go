// Package markdone is the command that marks an unmerged task done.
package markdone

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/IgorBolotnikov/DRUDGE/internal/task"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/actions"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/views"
)

const (
	verdictFormat          = "%s done"
	unblockedVerdictFormat = " · %s runnable: %s"
	unblockedSeparator     = ", "
)

// Kind marks a task done the way drg task done does. Its verdict names the
// tasks that became runnable.
var Kind = actions.Kind{Name: views.MarkDoneCommand, Run: run}

func run(services actions.Services, projectSlug string, command actions.Command, msg views.CommandMsg) tea.Msg {
	taskID := msg.(views.MarkDoneMsg).TaskID
	done := actions.DoneMsg{Command: command, Feeds: []views.FeedName{views.TasksFeed}}
	_, unblocked, err := services.Drudgers.MarkDone(projectSlug, taskID)
	if err != nil {
		done.Err = err
		return done
	}
	done.Verdict = verdict(command, unblocked)
	return done
}

func verdict(command actions.Command, unblocked []*task.Task) string {
	doneVerdict := fmt.Sprintf(verdictFormat, command.Target)
	if len(unblocked) == 0 {
		return doneVerdict
	}
	shortIDs := make([]string, 0, len(unblocked))
	for _, dependent := range unblocked {
		shortIDs = append(shortIDs, task.ShortID(dependent.ID))
	}
	return doneVerdict + fmt.Sprintf(unblockedVerdictFormat, formatTaskCount(len(unblocked)), strings.Join(shortIDs, unblockedSeparator))
}

func formatTaskCount(count int) string {
	if count == 1 {
		return "1 task"
	}
	return fmt.Sprintf("%d tasks", count)
}
