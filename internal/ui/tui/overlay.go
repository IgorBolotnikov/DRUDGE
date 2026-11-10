package tui

import (
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/IgorBolotnikov/DRUDGE/internal/task"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/styles"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/views/taskdetail"
)

// helpOverlay is the box with every key of the screen, drawn over the Tabs.
// While it is open it takes every key.
type helpOverlay struct {
	isOpen bool
}

func (overlay *helpOverlay) open() {
	overlay.isOpen = true
}

// TakesAllKeys reports whether the overlay is open.
func (overlay *helpOverlay) TakesAllKeys() bool {
	return overlay.isOpen
}

// Update closes the overlay on esc and on the help key. Every other key does
// nothing.
func (overlay *helpOverlay) Update(msg tea.Msg) tea.Cmd {
	keyPress, ok := msg.(tea.KeyPressMsg)
	if ok && key.Matches(keyPress, closeHelpKey, helpKey) {
		overlay.isOpen = false
	}
	return nil
}

// Space the task overlay leaves around it on the screen of the Tab.
const (
	taskOverlayMarginX = 2
	taskOverlayMarginY = 1
)

// taskOverlay is a Tile with a task detail, drawn over the active Tab. While
// it is open it takes every key. It never takes focus from a Tile of the Tab.
type taskOverlay struct {
	view   *taskdetail.View
	tile   *Tile
	isOpen bool
}

func newTaskOverlay(overlayStyles *styles.Styles) taskOverlay {
	view := taskdetail.New(overlayStyles)
	tile := NewTile(view, "", overlayStyles, defaultWeight, true)
	tile.SetFocused(true)
	return taskOverlay{view: view, tile: tile}
}

// open shows the task from the top of its description.
func (overlay *taskOverlay) open(id task.TaskID) {
	overlay.view.ShowTask(id)
	overlay.isOpen = true
}

// SetSize fits the overlay in the space of a Tab within the margins.
func (overlay *taskOverlay) SetSize(tabWidth, tabHeight int) {
	overlay.tile.SetSize(max(tabWidth-2*taskOverlayMarginX, 0), max(tabHeight-2*taskOverlayMarginY, 0))
}

// TakesAllKeys reports whether the overlay is open.
func (overlay *taskOverlay) TakesAllKeys() bool {
	return overlay.isOpen
}

// Update closes the overlay on esc and gives every other key to its View.
func (overlay *taskOverlay) Update(msg tea.Msg) tea.Cmd {
	keyPress, ok := msg.(tea.KeyPressMsg)
	if ok && key.Matches(keyPress, closeTaskKey) {
		overlay.isOpen = false
		return nil
	}
	return overlay.view.Update(msg)
}
