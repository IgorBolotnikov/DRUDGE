// Package reclaim is the command that frees the Drudger slots whose agent is
// gone.
package reclaim

import (
	"fmt"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/IgorBolotnikov/DRUDGE/internal/drudger"
	"github.com/IgorBolotnikov/DRUDGE/internal/task"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/actions"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/views"
)

const (
	oneFreedVerdictFormat  = "freed slot %s · rerun %s to start it over"
	manyFreedVerdictFormat = "freed slots %s · rerun %s to start them over"
	nothingFreedVerdict    = "no slot was claimed by a gone agent"
	freedListSeparator     = ", "
)

// Kind frees the Drudger slots whose agent is gone the way drg drudger
// reclaim does. Its verdict names the freed slots and the tasks they held.
var Kind = actions.Kind{Name: views.ReclaimCommand, Run: run}

func run(services actions.Services, projectSlug string, command actions.Command, msg views.CommandMsg) tea.Msg {
	done := actions.DoneMsg{Command: command, Feeds: []views.FeedName{views.DrudgersFeed}}
	freed, err := services.Drudgers.ReclaimDrudgers(projectSlug)
	if err != nil {
		done.Err = err
		return done
	}
	if len(freed) == 0 {
		done.Verdict = nothingFreedVerdict
		done.IsSkipped = true
		return done
	}
	done.Verdict = verdict(freed)
	return done
}

func verdict(freed []drudger.FreedSlot) string {
	slots := make([]string, 0, len(freed))
	shortIDs := make([]string, 0, len(freed))
	for _, entry := range freed {
		slots = append(slots, strconv.Itoa(entry.Slot))
		shortIDs = append(shortIDs, task.ShortID(entry.TaskID))
	}
	verdictFormat := manyFreedVerdictFormat
	if len(freed) == 1 {
		verdictFormat = oneFreedVerdictFormat
	}
	return fmt.Sprintf(verdictFormat, strings.Join(slots, freedListSeparator), strings.Join(shortIDs, freedListSeparator))
}
