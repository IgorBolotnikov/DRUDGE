// Package feeds runs the feeds that carry data to the Views of the TUI. It
// holds what every feed shares: the description of a kind of feed, the
// services feeds read from and the Runner. Each kind of feed lives in a
// package of its own below it.
package feeds

import (
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/IgorBolotnikov/DRUDGE/internal/common"
	"github.com/IgorBolotnikov/DRUDGE/internal/drudger"
	"github.com/IgorBolotnikov/DRUDGE/internal/task"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/views"
)

// TaskSource lists the tasks of a project.
type TaskSource interface {
	ListTasks(projectSlug string, filter task.ListTasksFilter, page int, size int) (common.Page[task.ListedTask], error)
}

// DrudgerSource reads the Drudgers of a project as the Drudger file records
// them, checks them the way drg drudger list does, records the outcome of
// every finished Session the way drg task status does and reads the latest
// run of a task the way drg task runlog does.
type DrudgerSource interface {
	ReadDrudgers(projectSlug string) ([]*drudger.Drudger, error)
	ListDrudgers(projectSlug string, page int, size int) (common.Page[*drudger.Drudger], error)
	RecordFinishedSessions(projectSlug string) ([]task.TaskID, error)
	RunStepsFrom(projectSlug string, requestedID task.TaskID, cursor drudger.RunCursor) (*drudger.RunStepsRead, error)
}

// Sources are the services feeds read their data from.
type Sources struct {
	Tasks    TaskSource
	Drudgers DrudgerSource
}

// RunFunc runs a feed once. It returns the message for the Views, the state
// the next run of the feed gets and the error the run failed with. A nil
// message reaches no View. The first run of a feed gets a nil state.
type RunFunc func(sources Sources, projectSlug string, feed views.Feed, state any) (msg tea.Msg, next any, err error)

// Kind describes one kind of feed.
type Kind struct {
	// Name is also the key of the interval of the feed in the TUI config file.
	Name views.FeedName
	// Description says in one line what the feed reads.
	Description     string
	DefaultInterval time.Duration
	MinInterval     time.Duration
	Run             RunFunc
}
