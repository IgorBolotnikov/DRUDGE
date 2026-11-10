package tui

import tea "charm.land/bubbletea/v2"

// layoutPath is the path of the layout of a Tab in the errors about it.
const layoutPath = "layout"

// Tab is a full screen with a title and a layout of Tiles. One Tile of the Tab
// has focus. A zoomed Tab gives all of its space to the focused Tile.
type Tab struct {
	Title            string
	layout           Node
	tiles            []*Tile
	focusedTileIndex int
	isZoomed         bool
	width            int
	height           int
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
	tab.width = width
	tab.height = height
	tab.layout.SetSize(width, height)
	if tab.isZoomed {
		tab.FocusedTile().SetSize(width, height)
	}
}

// FocusedTile returns the Tile with focus.
func (tab *Tab) FocusedTile() *Tile {
	return tab.tiles[tab.focusedTileIndex]
}

// IsZoomed reports whether the focused Tile fills the Tab.
func (tab *Tab) IsZoomed() bool {
	return tab.isZoomed
}

// ToggleZoom makes the focused Tile fill the Tab, or restores the layout when
// it already does.
func (tab *Tab) ToggleZoom() {
	tab.isZoomed = !tab.isZoomed
	tab.FocusedTile().SetZoomed(tab.isZoomed)
	tab.SetSize(tab.width, tab.height)
}

// FocusToward moves focus to the nearest Tile in a direction on the screen
// of the tiled layout. It keeps focus when no Tile lies that way.
func (tab *Tab) FocusToward(direction Direction) {
	rects := TileRects(tab.layout, tab.width, tab.height)
	tab.focus(NearestTile(rects, tab.focusedTileIndex, direction))
}

// FocusNextTile moves focus to the next Tile in tree order. The last Tile
// wraps to the first.
func (tab *Tab) FocusNextTile() {
	nextIndex := tab.focusedTileIndex + 1
	if nextIndex == len(tab.tiles) {
		nextIndex = 0
	}
	tab.focus(nextIndex)
}

// FocusPreviousTile moves focus to the previous Tile in tree order. The first
// Tile wraps to the last.
func (tab *Tab) FocusPreviousTile() {
	previousIndex := tab.focusedTileIndex - 1
	if previousIndex < 0 {
		previousIndex = len(tab.tiles) - 1
	}
	tab.focus(previousIndex)
}

// focus gives focus to the Tile at index. A zoomed Tab zooms that Tile in
// place of the one that had focus.
func (tab *Tab) focus(index int) {
	tab.FocusedTile().SetFocused(false)
	tab.FocusedTile().SetZoomed(false)
	tab.focusedTileIndex = index
	tab.FocusedTile().SetFocused(true)
	tab.FocusedTile().SetZoomed(tab.isZoomed)
	tab.SetSize(tab.width, tab.height)
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
// out the empty ones. A zoomed Tab returns only the segment of the focused
// Tile.
func (tab *Tab) Statuses() []string {
	shownTiles := tab.tiles
	if tab.isZoomed {
		shownTiles = []*Tile{tab.FocusedTile()}
	}
	var statuses []string
	for _, tile := range shownTiles {
		if status := tile.Status(); status != "" {
			statuses = append(statuses, status)
		}
	}
	return statuses
}

// HasRoomForTiles reports whether every Tile shown got space for its View at
// the size the Tab last got. A zoomed Tab shows only the focused Tile.
func (tab *Tab) HasRoomForTiles() bool {
	if tab.isZoomed {
		return tab.FocusedTile().hasRoomForView()
	}
	for _, tile := range tab.tiles {
		if !tile.hasRoomForView() {
			return false
		}
	}
	return true
}

// Render returns the lines of the Tab.
func (tab *Tab) Render() string {
	if tab.isZoomed {
		return tab.FocusedTile().Render()
	}
	return tab.layout.Render()
}
