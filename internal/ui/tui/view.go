package tui

import (
	"charm.land/bubbles/v2/key"
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
	View() string
	// KeyHints are the keys the bottom bar shows while the Tile of the View
	// has focus.
	KeyHints() []key.Binding
}

// tasksLoadedMsg carries the tasks of the project, or the error listing them
// failed with.
type tasksLoadedMsg struct {
	tasks []task.ListedTask
	err   error
}
