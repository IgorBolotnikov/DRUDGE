package views

import "github.com/IgorBolotnikov/DRUDGE/internal/task"

// TasksLoadedMsg carries the tasks of the project, or the error listing them
// failed with.
type TasksLoadedMsg struct {
	Tasks []task.ListedTask
	Err   error
}
