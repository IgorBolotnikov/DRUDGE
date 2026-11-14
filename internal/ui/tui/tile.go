package tui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/cells"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/styles"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/views"
)

// tileBorder is the width and the height a Tile border takes.
const tileBorder = 2

// tileEdge is the width of one side of a Tile border.
const tileEdge = 1

// The smallest space inside its border a Tile shows its View in.
const (
	minViewWidth  = 10
	minViewHeight = 3
)

// Markers in the title of a Tile. The focus marker keeps focus visible with
// color off.
const (
	focusMarker = "▸ "
	zoomMarker  = " [zoom]"
)

// Tile is one rectangle of a Tab and a leaf of its layout. It draws a border
// with its title in the top edge and the status of its View in the bottom
// edge, and clips the View to the space inside.
type Tile struct {
	view          views.View
	title         string
	styles        *styles.Styles
	weight        float64
	isStatusShown bool
	isFocused     bool
	isZoomed      bool
	width         int
	height        int
}

// NewTile places a View in a Tile. NewTab checks the weight.
func NewTile(view views.View, title string, tileStyles *styles.Styles, weight float64, isStatusShown bool) *Tile {
	return &Tile{view: view, title: title, styles: tileStyles, weight: weight, isStatusShown: isStatusShown}
}

// Title returns the title of the Tile. A Tile with an empty title shows the
// title of its View, or the name of a View with no title.
func (tile *Tile) Title() string {
	if tile.title != "" {
		return tile.title
	}
	if titler, ok := tile.view.(views.Titler); ok {
		return titler.Title()
	}
	return tile.view.Name()
}

// Status returns the status segment of the View, or an empty string when the
// Tile hides it.
func (tile *Tile) Status() string {
	if !tile.isStatusShown {
		return ""
	}
	return tile.view.Status()
}

func (tile *Tile) Weight() float64 {
	return tile.weight
}

func (tile *Tile) Tiles() []*Tile {
	return []*Tile{tile}
}

// SetSize sets the outer size of the Tile and gives the View the space inside
// the border.
func (tile *Tile) SetSize(width, height int) {
	tile.width = width
	tile.height = height
	tile.view.SetSize(max(width-tileBorder, 0), max(height-tileBorder, 0))
}

// hasRoomForView reports whether the space inside the border is big enough
// for a View.
func (tile *Tile) hasRoomForView() bool {
	return tile.width-tileBorder >= minViewWidth && tile.height-tileBorder >= minViewHeight
}

// viewMouse moves a mouse event at a cell of the Tab into the cells of the
// View of the Tile at rect. ok is false for a cell on the border.
func viewMouse(msg tea.MouseMsg, rect Rect) (moved tea.MouseMsg, ok bool) {
	viewRect := Rect{
		X:      rect.X + tileEdge,
		Y:      rect.Y + tileEdge,
		Width:  rect.Width - tileBorder,
		Height: rect.Height - tileBorder,
	}
	mouse := msg.Mouse()
	if !viewRect.contains(mouse.X, mouse.Y) {
		return nil, false
	}
	return moveMouse(msg, -viewRect.X, -viewRect.Y), true
}

// SetFocused sets whether the Tile has focus.
func (tile *Tile) SetFocused(isFocused bool) {
	tile.isFocused = isFocused
}

// SetZoomed sets whether the Tile fills its Tab.
func (tile *Tile) SetZoomed(isZoomed bool) {
	tile.isZoomed = isZoomed
}

// Render returns exactly the lines of the Tile at its size.
func (tile *Tile) Render() string {
	if tile.width < tileBorder || tile.height < tileBorder {
		return cells.BlankLines(tile.width, tile.height)
	}
	border := lipgloss.RoundedBorder()
	innerWidth := tile.width - tileBorder
	innerHeight := tile.height - tileBorder

	lines := make([]string, 0, tile.height)
	lines = append(lines, tile.renderTopEdge(border, innerWidth))
	viewLines := strings.Split(tile.view.Render(), "\n")
	borderStyle := tile.borderStyle()
	left := borderStyle.Render(border.Left)
	right := borderStyle.Render(border.Right)
	for index := range innerHeight {
		line := ""
		if index < len(viewLines) {
			line = viewLines[index]
		}
		lines = append(lines, left+cells.Fit(line, innerWidth)+right)
	}
	lines = append(lines, tile.renderBottomEdge(border, innerWidth))
	return strings.Join(lines, "\n")
}

// renderBottomEdge returns the bottom border with the status of the Tile in
// it, one border cell before the corner. A status wider than the edge is cut
// from the start.
func (tile *Tile) renderBottomEdge(border lipgloss.Border, innerWidth int) string {
	borderStyle := tile.borderStyle()
	status := tile.Status()
	if status == "" {
		return borderStyle.Render(border.BottomLeft + strings.Repeat(border.Bottom, innerWidth) + border.BottomRight)
	}
	trail := ansi.Truncate(border.Bottom, innerWidth, "")
	trailWidth := ansi.StringWidth(trail)
	status = cutStart(" "+status+" ", innerWidth-trailWidth)
	statusWidth := ansi.StringWidth(status)
	rest := strings.Repeat(border.Bottom, innerWidth-trailWidth-statusWidth)
	return borderStyle.Render(border.BottomLeft+rest) +
		tile.styles.Muted.Render(status) +
		borderStyle.Render(trail+border.BottomRight)
}

// cutStart cuts cells from the start of text until it fits in width.
func cutStart(text string, width int) string {
	overflow := ansi.StringWidth(text) - width
	if overflow <= 0 {
		return text
	}
	return ansi.TruncateLeft(text, overflow, "")
}

// borderStyle returns the style of the border. A focused Tile takes the focus
// border.
func (tile *Tile) borderStyle() lipgloss.Style {
	if tile.isFocused {
		return tile.styles.FocusBorder
	}
	return tile.styles.Border
}

// renderTopEdge returns the top border with the title in it, one border cell
// after the corner. Only the title takes the title style. The title of a
// focused Tile starts with the focus marker and the title of a zoomed Tile
// ends with the zoom marker. A title wider than the edge is cut.
func (tile *Tile) renderTopEdge(border lipgloss.Border, innerWidth int) string {
	borderStyle := tile.borderStyle()
	titleStyle := borderStyle
	title := tile.Title()
	if tile.isFocused {
		titleStyle = tile.styles.TileTitle
		title = focusMarker + title
	}
	if tile.isZoomed {
		title += zoomMarker
	}
	lead := ansi.Truncate(border.Top, innerWidth, "")
	leadWidth := ansi.StringWidth(lead)
	title = ansi.Truncate(" "+title+" ", innerWidth-leadWidth, "")
	titleWidth := ansi.StringWidth(title)
	rest := strings.Repeat(border.Top, innerWidth-leadWidth-titleWidth)
	return borderStyle.Render(border.TopLeft+lead) +
		titleStyle.Render(title) +
		borderStyle.Render(rest+border.TopRight)
}
