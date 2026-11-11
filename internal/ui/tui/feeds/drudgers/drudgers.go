// Package drudgers is the feed that reads the Drudgers of a project as the
// Drudger file records them.
package drudgers

import (
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/feeds"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/views"
)

// Kind reads the Drudgers of the project. It runs no sandbox command.
var Kind = feeds.Kind{
	Name:            views.DrudgersFeed,
	Description:     "How often drg tui reads the Drudgers as DRUDGE last recorded them. A read asks no sandbox.",
	DefaultInterval: 2 * time.Second,
	MinInterval:     100 * time.Millisecond,
	Run:             run,
}

func run(sources feeds.Sources, projectSlug string, feed views.Feed, state any) (tea.Msg, any, error) {
	read, err := sources.Drudgers.ReadDrudgers(projectSlug)
	return views.DrudgersLoadedMsg{ProjectSlug: projectSlug, Drudgers: read, ReadAt: time.Now(), Err: err}, nil, err
}
