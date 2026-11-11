package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
)

// moveMouse returns a mouse event moved by a number of columns and rows, as
// the same kind of mouse message. It panics on a mouse message Bubble Tea does
// not define.
func moveMouse(msg tea.MouseMsg, columns, rows int) tea.MouseMsg {
	mouse := msg.Mouse()
	mouse.X += columns
	mouse.Y += rows
	switch msg.(type) {
	case tea.MouseClickMsg:
		return tea.MouseClickMsg(mouse)
	case tea.MouseReleaseMsg:
		return tea.MouseReleaseMsg(mouse)
	case tea.MouseWheelMsg:
		return tea.MouseWheelMsg(mouse)
	case tea.MouseMotionMsg:
		return tea.MouseMotionMsg(mouse)
	}
	panic(fmt.Sprintf("unknown mouse message %T", msg))
}
