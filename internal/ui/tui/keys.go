package tui

import "charm.land/bubbles/v2/key"

// Keys of the root. Views never bind them.
var (
	helpKey         = key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help"))
	quitKey         = key.NewBinding(key.WithKeys("q"), key.WithHelp("q", "quit"))
	interruptKey    = key.NewBinding(key.WithKeys("ctrl+c"))
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
	eventLogKey     = key.NewBinding(key.WithKeys("!"), key.WithHelp("!", "event log"))
	clearErrorKey   = key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "clear error"))
)

// closeOverlayKey closes the overlay on top.
var closeOverlayKey = key.NewBinding(key.WithKeys("esc", "q"), key.WithHelp("esc/q", "close"))

// globalHelpTitle heads the keys of the root in the help.
const globalHelpTitle = "Global"

// helpOverlayTitle is the title of the help among the open overlays.
const helpOverlayTitle = "Help"

// rootBindings are the root keys the help lists. A key that carries the help of
// its pair stands for both.
var rootBindings = []key.Binding{zoomKey, tabNumberKey, nextTabKey, focusLeftKey, nextTileKey, eventLogKey, clearErrorKey, helpKey, quitKey}

// overlayBindings are the keys of the root the help lists while an overlay is
// on top.
var overlayBindings = []key.Binding{closeOverlayKey, helpKey}
