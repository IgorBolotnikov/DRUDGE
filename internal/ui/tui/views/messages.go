package views

import (
	"time"

	"github.com/IgorBolotnikov/DRUDGE/internal/drudger"
	"github.com/IgorBolotnikov/DRUDGE/internal/task"
)

// TasksLoadedMsg carries the tasks of a project, or the error listing them
// failed with.
type TasksLoadedMsg struct {
	ProjectSlug string
	Tasks       []task.ListedTask
	Err         error
}

// DrudgersLoadedMsg carries the Drudgers of a project as the Drudger file
// records them and when they were read, or the error reading them failed with.
type DrudgersLoadedMsg struct {
	ProjectSlug string
	Drudgers    []*drudger.Drudger
	ReadAt      time.Time
	Err         error
}

// DrudgersCheckedMsg says when a check of the Drudgers of a project ended, or
// carries the error it failed with.
type DrudgersCheckedMsg struct {
	ProjectSlug string
	CheckedAt   time.Time
	Err         error
}

// TaskSelectedMsg tells the Tab of a View that the cursor of the View moved
// onto a task.
type TaskSelectedMsg struct {
	TaskID task.TaskID
}

// OpenTaskMsg asks the Tab of a View to show a task in a View that can show
// one.
type OpenTaskMsg struct {
	TaskID task.TaskID
}
