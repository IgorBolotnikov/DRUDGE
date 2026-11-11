// Package sessions is the feed that records the outcome of every finished
// Session of a project.
package sessions

import (
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/feeds"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/views"
)

// Kind checks the Session of every in-progress task of the project and
// records the outcome of each one that finished, with close-out and pull
// requests.
var Kind = feeds.Kind{
	Name:            views.SessionsFeed,
	Description:     "How often drg tui checks the Sessions of the in-progress tasks and records the outcome of each finished one with what drg task status does.",
	DefaultInterval: 5 * time.Second,
	MinInterval:     100 * time.Millisecond,
	Run:             run,
}

func run(sources feeds.Sources, projectSlug string, feed views.Feed, state any) (tea.Msg, any, error) {
	changedIDs, err := sources.Drudgers.RecordFinishedSessions(projectSlug)
	return views.SessionsCheckedMsg{ProjectSlug: projectSlug, ChangedTaskIDs: changedIDs, Err: err}, nil, err
}
