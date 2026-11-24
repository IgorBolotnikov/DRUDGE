// Package pullrequests is the command that opens the pull requests close-out
// left behind for a task.
package pullrequests

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/IgorBolotnikov/DRUDGE/internal/drudger"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/actions"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/views"
)

const (
	noneLeftVerdictFormat = "no pull requests of %s left to open"
	openedVerdictFormat   = "opened %s of %s"
	partialVerdictFormat  = "opened %d of %d pull requests of %s"
)

// Kind opens the pull requests of a task the way drg task pr does. Its
// verdict counts the pull requests it opened.
var Kind = actions.Kind{Name: views.PullRequestsCommand, Run: run}

func run(services actions.Services, projectSlug string, command actions.Command, msg views.CommandMsg) tea.Msg {
	taskID := msg.(views.PullRequestsMsg).TaskID
	done := actions.DoneMsg{Command: command, Feeds: []views.FeedName{views.TasksFeed}}
	retried, err := services.Drudgers.RetryPullRequests(projectSlug, taskID)
	if err != nil {
		done.Err = err
		return done
	}
	done.Verdict = verdict(command, retried)
	done.IsSkipped = retried.Opened == 0
	return done
}

func verdict(command actions.Command, retried drudger.PullRequestsRetried) string {
	switch {
	case retried.Tried == 0:
		return fmt.Sprintf(noneLeftVerdictFormat, command.Target)
	case retried.Opened == retried.Tried:
		return fmt.Sprintf(openedVerdictFormat, formatPullRequestCount(retried.Opened), command.Target)
	default:
		return fmt.Sprintf(partialVerdictFormat, retried.Opened, retried.Tried, command.Target)
	}
}

func formatPullRequestCount(count int) string {
	if count == 1 {
		return "1 pull request"
	}
	return fmt.Sprintf("%d pull requests", count)
}
