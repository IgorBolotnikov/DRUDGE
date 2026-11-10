package tui

import tea "charm.land/bubbletea/v2"

// layoutPath is the path of the layout of a Tab in the errors about it.
const layoutPath = "layout"

// Tab is a full screen with a title and a layout of Tiles. One Tile of the Tab
// has focus.
type Tab struct {
	Title            string
	layout           Node
	tiles            []*Tile
	focusedTileIndex int
}

// NewTab returns a Tab with a layout and gives focus to its first Tile. It
// refuses a layout it cannot divide, with an error that starts with the path
// of the value to change, like layout.children[1].weight.
func NewTab(title string, layout Node) (*Tab, error) {
	if err := validateNode(layout, layoutPath); err != nil {
		return nil, err
	}
	tiles := layout.Tiles()
	tiles[0].SetFocused(true)
	return &Tab{Title: title, layout: layout, tiles: tiles}, nil
}

// SetSize gives the Tab the space between the top and the bottom bar.
func (tab *Tab) SetSize(width, height int) {
	tab.layout.SetSize(width, height)
}

// FocusedTile returns the Tile with focus.
func (tab *Tab) FocusedTile() *Tile {
	return tab.tiles[tab.focusedTileIndex]
}

// Broadcast sends a message to the View of every Tile.
func (tab *Tab) Broadcast(msg tea.Msg) tea.Cmd {
	commands := make([]tea.Cmd, 0, len(tab.tiles))
	for _, tile := range tab.tiles {
		commands = append(commands, tile.view.Update(msg))
	}
	return tea.Batch(commands...)
}

// Statuses returns the status segments of the Tiles in tree order. It leaves
// out the empty ones.
func (tab *Tab) Statuses() []string {
	var statuses []string
	for _, tile := range tab.tiles {
		if status := tile.Status(); status != "" {
			statuses = append(statuses, status)
		}
	}
	return statuses
}

// HasRoomForTiles reports whether every Tile got space for its View at the
// size the Tab last got.
func (tab *Tab) HasRoomForTiles() bool {
	for _, tile := range tab.tiles {
		if !tile.hasRoomForView() {
			return false
		}
	}
	return true
}

// Render returns the lines of the Tab.
func (tab *Tab) Render() string {
	return tab.layout.Render()
}
