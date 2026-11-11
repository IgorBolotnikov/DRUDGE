// Package tasks is the feed that lists the tasks of a project.
package tasks

import (
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/IgorBolotnikov/DRUDGE/internal/task"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/feeds"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/views"
)

// The feed lists every task of the project on one page.
const (
	firstPage    = 1
	allOnOnePage = 0
)

// Kind lists the tasks of the project.
var Kind = feeds.Kind{
	Name:            views.TasksFeed,
	Description:     "How often drg tui lists the tasks.",
	DefaultInterval: 2 * time.Second,
	MinInterval:     100 * time.Millisecond,
	Run:             run,
}

func run(sources feeds.Sources, projectSlug string, feed views.Feed, state any) (tea.Msg, any, error) {
	listed, err := sources.Tasks.ListTasks(projectSlug, task.ListTasksFilter{}, firstPage, allOnOnePage)
	return views.TasksLoadedMsg{ProjectSlug: projectSlug, Tasks: listed.Items, Err: err}, nil, err
}
