// Package views holds what the View of every Tile has in common: the View
// interface, the options of a View, the feeds a View subscribes to and the
// messages that carry data to Views. Each View lives in a package of its own below it.
package views

import (
	tea "charm.land/bubbletea/v2"

	"github.com/IgorBolotnikov/DRUDGE/internal/task"
)

// View is what a Tile shows. It renders within the size it last got and
// never calls a service. The root model sends it the data of the feeds it
// subscribes to as messages.
type View interface {
	// Name is the kind of the View, shown as the title of its Tile.
	Name() string
	SetSize(width, height int)
	Update(msg tea.Msg) tea.Cmd
	Render() string
	// TakesAllKeys reports whether the View gets every key, the keys of the
	// root among them, like while a filter input is open.
	TakesAllKeys() bool
	// Status is a short plain-text segment the Tile of the View shows in its
	// bottom border. An empty string shows nothing.
	Status() string
}

// TaskShower is a View that shows one task. Its Tab hands it the selected
// task.
type TaskShower interface {
	View
	ShowTask(id task.TaskID)
}

// Opener is a View that can show what some open messages ask for.
type Opener interface {
	View
	// Open shows what msg asks for. isShown is false for a message the View
	// can't show.
	Open(msg OpenMsg) (isShown bool)
}

// Titler is a View that titles its Tile. A Tile with a title of its own
// keeps it.
type Titler interface {
	View
	// Title follows what the View shows.
	Title() string
}

// Subscriber is a View that shows the data of feeds.
type Subscriber interface {
	View
	// Feeds returns the feeds the View shows right now.
	Feeds() []Feed
}
