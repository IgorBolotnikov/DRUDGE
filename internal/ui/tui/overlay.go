package tui

import (
	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/styles"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/views"
)

// Overlay is a box drawn over the screen. Overlays stack, and only the top one
// gets keys and the mouse.
type Overlay interface {
	// SetSize gives the overlay the size of the whole screen.
	SetSize(screenWidth, screenHeight int)
	Update(msg tea.Msg) tea.Cmd
	// Render returns the box and the cell of the screen its top left corner
	// sits on. An empty box draws nothing.
	Render() (box string, x, y int)
	// Mouse gets mouse events in the cells of the screen. The overlay ignores
	// the ones outside its box.
	Mouse(msg tea.MouseMsg) tea.Cmd
	// Title heads the keys of the overlay in the help.
	Title() string
	// Keys are the keys the help lists while the overlay is on top.
	Keys() []key.Binding
	// ShortHelp is the hint the bottom bar shows while the overlay is on top.
	ShortHelp() []key.Binding
	// TakesAllKeys reports whether the overlay gets every key but ctrl+c, esc
	// and ? included, like while a text input is open.
	TakesAllKeys() bool
	// Feeds returns the feeds the overlay shows.
	Feeds() []views.Feed
}

// overlayMsg is a message of a command of an overlay.
type overlayMsg struct {
	msg tea.Msg
}

// tagOverlay returns a command that runs cmd and wraps its message as a
// message of an overlay.
func tagOverlay(cmd tea.Cmd) tea.Cmd {
	if cmd == nil {
		return nil
	}
	return func() tea.Msg {
		return overlayMsg{msg: cmd()}
	}
}

// Space a View overlay leaves around it in the space of a Tab.
const (
	viewOverlayMarginX = 2
	viewOverlayMarginY = 1
)

// viewOverlay is a Tile with a View, drawn over the space of the Tab. Its Tile
// always has focus. It never takes focus from a Tile of the Tab.
type viewOverlay struct {
	tile         *Tile
	screenHeight int
}

func newViewOverlay(view views.View, overlayStyles *styles.Styles) *viewOverlay {
	tile := NewTile(view, "", overlayStyles, defaultWeight, true)
	tile.SetFocused(true)
	return &viewOverlay{tile: tile}
}

// SetSize fits the Tile in the space of a Tab within the margins.
func (overlay *viewOverlay) SetSize(screenWidth, screenHeight int) {
	overlay.screenHeight = screenHeight
	tabHeight := max(screenHeight-barHeight, 0)
	overlay.tile.SetSize(max(screenWidth-2*viewOverlayMarginX, 0), max(tabHeight-2*viewOverlayMarginY, 0))
}

func (overlay *viewOverlay) Update(msg tea.Msg) tea.Cmd {
	return overlay.tile.view.Update(msg)
}

// Render returns an empty box while the screen has no room for a Tab.
func (overlay *viewOverlay) Render() (box string, x, y int) {
	if overlay.screenHeight <= barHeight {
		return "", 0, 0
	}
	rect := overlay.rect()
	return overlay.tile.Render(), rect.X, rect.Y
}

// rect returns the cells of the screen the Tile takes.
func (overlay *viewOverlay) rect() Rect {
	return Rect{X: viewOverlayMarginX, Y: tabTop + viewOverlayMarginY, Width: overlay.tile.width, Height: overlay.tile.height}
}

func (overlay *viewOverlay) Mouse(msg tea.MouseMsg) tea.Cmd {
	inView, ok := viewMouse(msg, overlay.rect())
	if !ok {
		return nil
	}
	return overlay.tile.view.Update(inView)
}

func (overlay *viewOverlay) Title() string {
	return overlay.tile.Title()
}

// Keys returns the keys the registry lists for the View, or none for a View
// it does not know.
func (overlay *viewOverlay) Keys() []key.Binding {
	kind, _ := findViewKind(overlay.tile.view.Name())
	return kind.keys
}

func (overlay *viewOverlay) ShortHelp() []key.Binding {
	return []key.Binding{closeOverlayKey, helpKey}
}

func (overlay *viewOverlay) TakesAllKeys() bool {
	return overlay.tile.view.TakesAllKeys()
}

func (overlay *viewOverlay) Feeds() []views.Feed {
	return viewFeeds(overlay.tile.view)
}

// helpSection is one titled list of keys in the help.
type helpSection struct {
	title string
	keys  []key.Binding
}

// helpOverlay is the box that lists keys in titled sections one above
// another, centered over the whole screen. It ignores keys and the mouse, and the root
// closes it.
type helpOverlay struct {
	help         help.Model
	boxStyle     lipgloss.Style
	titleStyle   lipgloss.Style
	sections     []helpSection
	screenWidth  int
	screenHeight int
}

func newHelpOverlay(keyHelp help.Model, overlayStyles *styles.Styles, sections []helpSection) *helpOverlay {
	return &helpOverlay{
		help:       keyHelp,
		boxStyle:   overlayStyles.HelpBox,
		titleStyle: overlayStyles.HelpTitle,
		sections:   sections,
	}
}

func (overlay *helpOverlay) SetSize(screenWidth, screenHeight int) {
	overlay.screenWidth = screenWidth
	overlay.screenHeight = screenHeight
}

func (overlay *helpOverlay) Update(msg tea.Msg) tea.Cmd {
	return nil
}

// Render centers the box on the screen. A box bigger than the screen starts
// at the top left corner.
func (overlay *helpOverlay) Render() (box string, x, y int) {
	box = overlay.boxStyle.Render(overlay.renderSections())
	x = max((overlay.screenWidth-lipgloss.Width(box))/2, 0)
	y = max((overlay.screenHeight-lipgloss.Height(box))/2, 0)
	return box, x, y
}

// renderSections renders each section with keys as its title over its keys,
// with a blank line between two sections. A section without keys is left out.
func (overlay *helpOverlay) renderSections() string {
	var blocks []string
	for _, section := range overlay.sections {
		if len(section.keys) == 0 {
			continue
		}
		if len(blocks) > 0 {
			blocks = append(blocks, "")
		}
		blocks = append(blocks, overlay.titleStyle.Render(section.title), overlay.help.FullHelpView([][]key.Binding{section.keys}))
	}
	return lipgloss.JoinVertical(lipgloss.Left, blocks...)
}

func (overlay *helpOverlay) Title() string {
	return helpOverlayTitle
}

func (overlay *helpOverlay) Mouse(msg tea.MouseMsg) tea.Cmd {
	return nil
}

func (overlay *helpOverlay) Keys() []key.Binding {
	return nil
}

func (overlay *helpOverlay) ShortHelp() []key.Binding {
	return []key.Binding{closeOverlayKey}
}

func (overlay *helpOverlay) TakesAllKeys() bool {
	return false
}

func (overlay *helpOverlay) Feeds() []views.Feed {
	return nil
}
