package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/IgorBolotnikov/DRUDGE/internal/task"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/views"
)

// layoutPath is the path of the layout of a Tab in the errors about it.
const layoutPath = "layout"

// Tab is a full screen with a title and a layout of Tiles. One Tile of the Tab
// has focus. A zoomed Tab gives all of its space to the focused Tile. The Tab
// keeps the task selected in its Views and hands it to its Views that show a
// task.
type Tab struct {
	Title            string
	layout           Node
	tiles            []*Tile
	focusedTileIndex int
	isZoomed         bool
	isFocusHidden    bool
	selectedTaskID   task.TaskID
	width            int
	height           int
}

// tabMsg is a message of a command of a View, with the Tab of the View.
type tabMsg struct {
	tab *Tab
	msg tea.Msg
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

// SetFocusHidden sets whether the focused Tile draws as unfocused. An overlay
// over the Tab hides the focus of the Tab. The Tile keeps its focus.
func (tab *Tab) SetFocusHidden(isFocusHidden bool) {
	tab.isFocusHidden = isFocusHidden
	tab.FocusedTile().SetFocused(!isFocusHidden)
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
	tab.FocusedTile().SetFocused(!tab.isFocusHidden)
	tab.FocusedTile().SetZoomed(tab.isZoomed)
	tab.SetSize(tab.width, tab.height)
}

// Mouse passes a mouse event at a cell of the Tab to the View of the Tile
// there, focused or not, in the cells of the View. A click also gives focus to
// the Tile, and a click on its border only gives it focus.
func (tab *Tab) Mouse(msg tea.MouseMsg) tea.Cmd {
	mouse := msg.Mouse()
	tileIndex, rect, ok := tab.tileAt(mouse.X, mouse.Y)
	if !ok {
		return nil
	}
	if _, isClick := msg.(tea.MouseClickMsg); isClick {
		tab.focus(tileIndex)
	}
	inView, ok := viewMouse(msg, rect)
	if !ok {
		return nil
	}
	return tab.tag(tab.tiles[tileIndex].view.Update(inView))
}

// tileAt returns the index and the Rect of the Tile shown at a cell of the
// Tab. A zoomed Tab shows only the focused Tile. ok is false for a cell no
// Tile takes.
func (tab *Tab) tileAt(x, y int) (tileIndex int, rect Rect, ok bool) {
	if tab.isZoomed {
		rect = Rect{Width: tab.width, Height: tab.height}
		return tab.focusedTileIndex, rect, rect.contains(x, y)
	}
	for index, tileRect := range TileRects(tab.layout, tab.width, tab.height) {
		if tileRect.contains(x, y) {
			return index, tileRect, true
		}
	}
	return 0, Rect{}, false
}

// Broadcast sends a message to the View of every Tile.
func (tab *Tab) Broadcast(msg tea.Msg) tea.Cmd {
	commands := make([]tea.Cmd, 0, len(tab.tiles))
	for _, tile := range tab.tiles {
		commands = append(commands, tab.tag(tile.view.Update(msg)))
	}
	return tea.Batch(commands...)
}

// TakesAllKeys reports whether the View of the focused Tile takes all keys.
func (tab *Tab) TakesAllKeys() bool {
	return tab.FocusedTile().view.TakesAllKeys()
}

// Update sends a message to the View of the focused Tile.
func (tab *Tab) Update(msg tea.Msg) tea.Cmd {
	return tab.tag(tab.FocusedTile().view.Update(msg))
}

// tag returns a command that runs cmd and wraps its message with the Tab, so
// the root can tell which Tab a selection came from.
func (tab *Tab) tag(cmd tea.Cmd) tea.Cmd {
	if cmd == nil {
		return nil
	}
	return func() tea.Msg {
		return tabMsg{tab: tab, msg: cmd()}
	}
}

// SelectTask keeps the task as the selected task of the Tab and hands it to
// every View of the Tab that shows a task.
func (tab *Tab) SelectTask(id task.TaskID) {
	tab.selectedTaskID = id
	for _, tile := range tab.tiles {
		if shower, ok := tile.view.(views.TaskShower); ok {
			shower.ShowTask(id)
		}
	}
}

// SelectedTaskID returns the task last selected in the Tab, or an empty ID
// before any.
func (tab *Tab) SelectedTaskID() task.TaskID {
	return tab.selectedTaskID
}

// Open hands an open message to the View of every Tile that is an opener and
// gives focus to the Tile of the first View in tree order that shows it.
// isShown is false when no View of the Tab shows it.
func (tab *Tab) Open(msg views.OpenMsg) (isShown bool) {
	for index, tile := range tab.tiles {
		opener, ok := tile.view.(views.Opener)
		if !ok || !opener.Open(msg) {
			continue
		}
		if !isShown {
			tab.focus(index)
		}
		isShown = true
	}
	return isShown
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
