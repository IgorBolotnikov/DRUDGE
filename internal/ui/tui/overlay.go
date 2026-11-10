package tui

import (
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
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
