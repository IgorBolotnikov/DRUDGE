// Package drudgercheck is the feed that checks the Drudgers of a project the
// way drg drudger list does.
package drudgercheck

import (
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/feeds"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/views"
)

// The check lists every Drudger of the project on one page.
const (
	firstPage    = 1
	allOnOnePage = 0
)

// Kind checks the Drudgers of the project. A check asks every sandbox, so it
// runs less often than the other feeds.
var Kind = feeds.Kind{
	Name:            views.DrudgerCheckFeed,
	Description:     "How often drg tui checks the Drudgers with what drg drudger list does.",
	DefaultInterval: 30 * time.Second,
	MinInterval:     time.Second,
	Run:             run,
}

func run(sources feeds.Sources, projectSlug string, feed views.Feed, state any) (tea.Msg, any, error) {
	_, err := sources.Drudgers.ListDrudgers(projectSlug, firstPage, allOnOnePage)
	return views.DrudgersCheckedMsg{ProjectSlug: projectSlug, CheckedAt: time.Now(), Err: err}, nil, err
}
