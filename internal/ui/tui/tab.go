package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/styles"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/views/tasklist"
)

const tasksTabTitle = "Tasks"

// defaultTabs returns the Tabs the TUI shows.
func defaultTabs(screenStyles *styles.Styles) []*Tab {
	return []*Tab{NewTab(tasksTabTitle, NewTile(tasklist.New(screenStyles), screenStyles))}
}

// Tab is a full screen with a title and a layout of Tiles. Its layout is one
// Tile that fills it.
type Tab struct {
	Title string
	tile  *Tile
}

// NewTab returns a Tab that holds one Tile and gives it focus.
func NewTab(title string, tile *Tile) *Tab {
	tile.SetFocused(true)
	return &Tab{Title: title, tile: tile}
}

// SetSize gives the Tab the space between the top and the bottom bar.
func (tab *Tab) SetSize(width, height int) {
	tab.tile.SetSize(width, height)
}

// FocusedTile returns the Tile with focus.
func (tab *Tab) FocusedTile() *Tile {
	return tab.tile
}

// Broadcast sends a message to the View of every Tile.
func (tab *Tab) Broadcast(msg tea.Msg) tea.Cmd {
	return tab.tile.view.Update(msg)
}

// Render returns the lines of the Tab.
func (tab *Tab) Render() string {
	return tab.tile.Render()
}
