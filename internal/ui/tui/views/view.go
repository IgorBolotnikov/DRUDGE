// Package views holds what the View of every Tile has in common: the View
// interface, the options of a View and the messages that carry data to
// Views. Each View lives in a package of its own below it.
package views

import (
	tea "charm.land/bubbletea/v2"

	"github.com/IgorBolotnikov/DRUDGE/internal/task"
)

// View is what a Tile shows. It renders within the size it last got and
// never calls a service. The root model sends it the data it shows as
// messages.
type View interface {
	// Name is the kind of the View, shown as the title of its Tile.
	Name() string
	SetSize(width, height int)
	Update(msg tea.Msg) tea.Cmd
	Render() string
	// TakesAllKeys reports whether the View gets every key, the keys of the
	// root among them, like while a filter input is open.
	TakesAllKeys() bool
	// Status is a short plain-text segment the bottom bar shows for the Tile
	// of the View. An empty string shows nothing.
	Status() string
}

// TaskShower is a View that shows one task. Its Tab hands it the selected task
// and the task to open.
type TaskShower interface {
	View
	ShowTask(id task.TaskID)
}
