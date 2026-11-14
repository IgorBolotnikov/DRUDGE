// Package runlog is the feed that reads the latest run of one task the way
// drg task runlog does, a part at a time.
package runlog

import (
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/IgorBolotnikov/DRUDGE/internal/drudger"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/feeds"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/views"
)

// Kind reads the steps and the stderr lines the latest run of a task wrote
// since its last run. Its state is the run cursor. A failed read keeps the
// cursor, so the next run reads from the same spot. It keeps reading after
// the run finished to catch a rerun of the task.
var Kind = feeds.Kind{
	Name:            views.RunLogFeed,
	Description:     "How often drg tui reads what the run of a task shown in a runlog wrote since the last read.",
	DefaultInterval: time.Second,
	MinInterval:     100 * time.Millisecond,
	Run:             run,
}

func run(sources feeds.Sources, projectSlug string, feed views.Feed, state any) (tea.Msg, any, error) {
	cursor, _ := state.(drudger.RunCursor)
	read, err := sources.Drudgers.RunStepsFrom(projectSlug, feed.TaskID, cursor)
	msg := views.RunStepsReadMsg{TaskID: feed.TaskID, IsFirstRead: state == nil, Read: read, Err: err}
	if err != nil {
		return msg, state, err
	}
	return msg, read.Next, nil
}
