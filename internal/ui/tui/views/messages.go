package views

import "github.com/IgorBolotnikov/DRUDGE/internal/task"

// TasksLoadedMsg carries the tasks of a project, or the error listing them
// failed with.
type TasksLoadedMsg struct {
	ProjectSlug string
	Tasks       []task.ListedTask
	Err         error
}
