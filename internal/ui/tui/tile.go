package tui

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/cells"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/styles"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/views"
)

// tileBorder is the width and the height a Tile border takes.
const tileBorder = 2

// Tile is one rectangle of a Tab and a leaf of its layout. It draws a border
// with the name of its View in the top edge and clips the View to the space
// inside.
type Tile struct {
	view      views.View
	styles    *styles.Styles
	weight    float64
	isFocused bool
	width     int
	height    int
}

// NewTile places a View in a Tile. NewTab checks the weight.
func NewTile(view views.View, tileStyles *styles.Styles, weight float64) *Tile {
	return &Tile{view: view, styles: tileStyles, weight: weight}
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

// SetFocused sets whether the Tile has focus.
func (tile *Tile) SetFocused(isFocused bool) {
	tile.isFocused = isFocused
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
	left := tile.styles.Border.Render(border.Left)
	right := tile.styles.Border.Render(border.Right)
	for index := range innerHeight {
		line := ""
		if index < len(viewLines) {
			line = viewLines[index]
		}
		lines = append(lines, left+cells.Fit(line, innerWidth)+right)
	}
	bottom := border.BottomLeft + strings.Repeat(border.Bottom, innerWidth) + border.BottomRight
	lines = append(lines, tile.styles.Border.Render(bottom))
	return strings.Join(lines, "\n")
}

// renderTopEdge returns the top border with the name of the View in it. A name
// wider than the edge is cut.
func (tile *Tile) renderTopEdge(border lipgloss.Border, innerWidth int) string {
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
