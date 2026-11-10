package tui

import "charm.land/bubbles/v2/key"

// Keys of the root. Views never bind them.
var (
	helpKey         = key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help"))
	quitKey         = key.NewBinding(key.WithKeys("q"), key.WithHelp("q", "quit"))
	interruptKey    = key.NewBinding(key.WithKeys("ctrl+c"))
	refreshKey      = key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "refresh"))
	checkKey        = key.NewBinding(key.WithKeys("R"), key.WithHelp("R", "check drudgers"))
	tabNumberKey    = key.NewBinding(key.WithKeys("1", "2", "3", "4", "5", "6", "7", "8", "9"), key.WithHelp("1-9", "tab"))
	nextTabKey      = key.NewBinding(key.WithKeys("]"), key.WithHelp("[/]", "prev/next tab"))
	previousTabKey  = key.NewBinding(key.WithKeys("["))
	focusLeftKey    = key.NewBinding(key.WithKeys("ctrl+h", "ctrl+left"), key.WithHelp("ctrl+h/j/k/l", "move focus"))
	focusDownKey    = key.NewBinding(key.WithKeys("ctrl+j", "ctrl+down"))
	focusUpKey      = key.NewBinding(key.WithKeys("ctrl+k", "ctrl+up"))
	focusRightKey   = key.NewBinding(key.WithKeys("ctrl+l", "ctrl+right"))
	nextTileKey     = key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab/shift+tab", "next/prev tile"))
	previousTileKey = key.NewBinding(key.WithKeys("shift+tab"))
	zoomKey         = key.NewBinding(key.WithKeys("z"), key.WithHelp("z", "zoom"))
)

// closeHelpKey closes the help overlay, as the help key does.
var closeHelpKey = key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "close help"))

// closeTaskKey closes the task overlay.
var closeTaskKey = key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "close"))

// rootBindings are the root keys the help lists. A key that carries the help of
// its pair stands for both.
var rootBindings = []key.Binding{zoomKey, refreshKey, checkKey, tabNumberKey, nextTabKey, focusLeftKey, nextTileKey, helpKey, quitKey}

// keyMap is the keys of the screen while a Tile has focus: the keys of its
// View and the root keys.
type keyMap struct {
	viewKeys []key.Binding
}

// ShortHelp returns the help key alone, the one hint the bottom bar shows.
func (keys keyMap) ShortHelp() []key.Binding {
	return []key.Binding{helpKey}
}

// FullHelp returns a column with the keys of the View and a column with the
// root keys.
func (keys keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{keys.viewKeys, rootBindings}
}
