package tui

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

const ellipsis = "…"

// tileBorder is the width and the height a Tile border takes.
const tileBorder = 2

// Tile is one rectangle of a Tab. It draws a border with the name of its
// View in the top edge and clips the View to the space inside.
type Tile struct {
	view      View
	styles    *Styles
	isFocused bool
	width     int
	height    int
}

// NewTile places view in a Tile.
func NewTile(view View, styles *Styles) *Tile {
	return &Tile{view: view, styles: styles}
}

// SetSize sets the outer size of the Tile and gives the View the space inside
// the border.
func (tile *Tile) SetSize(width, height int) {
	tile.width = width
	tile.height = height
	tile.view.SetSize(max(width-tileBorder, 0), max(height-tileBorder, 0))
}

// SetFocused sets whether the Tile has focus.
func (tile *Tile) SetFocused(isFocused bool) {
	tile.isFocused = isFocused
}

// Render returns exactly the lines of the Tile at its size.
func (tile *Tile) Render() string {
	if tile.width < tileBorder || tile.height < tileBorder {
		return blankLines(tile.width, tile.height)
	}
	border := lipgloss.RoundedBorder()
	innerWidth := tile.width - tileBorder
	innerHeight := tile.height - tileBorder

	lines := make([]string, 0, tile.height)
	lines = append(lines, tile.topEdge(border, innerWidth))
	viewLines := strings.Split(tile.view.View(), "\n")
	left := tile.styles.Border.Render(border.Left)
	right := tile.styles.Border.Render(border.Right)
	for index := range innerHeight {
		line := ""
		if index < len(viewLines) {
			line = viewLines[index]
		}
		lines = append(lines, left+fit(line, innerWidth)+right)
	}
	bottom := border.BottomLeft + strings.Repeat(border.Bottom, innerWidth) + border.BottomRight
	lines = append(lines, tile.styles.Border.Render(bottom))
	return strings.Join(lines, "\n")
}

// topEdge returns the top border with the name of the View in it. A name
// wider than the edge is cut.
func (tile *Tile) topEdge(border lipgloss.Border, innerWidth int) string {
	titleStyle := tile.styles.Border
	if tile.isFocused {
		titleStyle = tile.styles.TileTitle
	}
	title := ansi.Truncate(border.Top+" "+tile.view.Name()+" ", innerWidth, "")
	titleWidth := ansi.StringWidth(title)
	return tile.styles.Border.Render(border.TopLeft) +
		titleStyle.Render(title) +
		tile.styles.Border.Render(strings.Repeat(border.Top, innerWidth-titleWidth)+border.TopRight)
}

// fit cuts text to width cells and pads it with spaces up to width.
func fit(text string, width int) string {
	return pad(ansi.Truncate(text, width, ""), width)
}

// fitEllipsis is fit that ends cut text with an ellipsis.
func fitEllipsis(text string, width int) string {
	return pad(ansi.Truncate(text, width, ellipsis), width)
}

func pad(text string, width int) string {
	return text + strings.Repeat(" ", max(width-ansi.StringWidth(text), 0))
}

// blankLines returns height lines of width spaces.
func blankLines(width, height int) string {
	if height <= 0 {
		return ""
	}
	line := strings.Repeat(" ", max(width, 0))
	return strings.TrimSuffix(strings.Repeat(line+"\n", height), "\n")
}
