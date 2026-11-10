// Package table lays out the columns of a list View in the width of its Tile.
// Columns that do not fit drop out, and one column takes the width that is
// left.
package table

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/cells"
)

// Gap separates two columns.
const Gap = "  "

// Column is one column of a table.
type Column struct {
	Title string
	// Priority orders the columns that drop when the table is too narrow. The
	// column with the lowest priority drops first.
	Priority int
	// MinWidth is the fewest cells the column takes.
	MinWidth int
	// IsRequired keeps the column at any width.
	IsRequired bool
	// IsFill makes the column take the width the other columns leave. A fill
	// column is required. A table has at most one.
	IsFill bool
}

// Layout is the columns of a table that fit in a width, with the width of
// each.
type Layout struct {
	columns []Column
	widths  []int
	isShown []bool
}

// NewLayout fits the columns in width cells. A column is as wide as its title
// and its widest cell, and at least its minimum width. Columns drop by
// priority until the rest fit, and the fill column takes the width left over.
// When the required columns alone do not fit, the layout is wider than width.
func NewLayout(columns []Column, rows [][]string, width int) Layout {
	widths := make([]int, len(columns))
	for index, column := range columns {
		widths[index] = column.MinWidth
		if column.IsFill {
			continue
		}
		widths[index] = max(widths[index], ansi.StringWidth(column.Title))
		for _, row := range rows {
			widths[index] = max(widths[index], ansi.StringWidth(row[index]))
		}
	}
	isShown := make([]bool, len(columns))
	for index := range isShown {
		isShown[index] = true
	}
	layout := Layout{columns: columns, widths: widths, isShown: isShown}
	for layout.Width() > width {
		index, ok := layout.nextToDrop()
		if !ok {
			break
		}
		layout.isShown[index] = false
	}
	if fillIndex, ok := layout.fillIndex(); ok {
		layout.widths[fillIndex] += max(width-layout.Width(), 0)
	}
	return layout
}

// Width returns how many cells a line of the table takes.
func (layout Layout) Width() int {
	width := 0
	shownCount := 0
	for index, columnWidth := range layout.widths {
		if layout.isShown[index] {
			width += columnWidth
			shownCount++
		}
	}
	if shownCount == 0 {
		return 0
	}
	return width + (shownCount-1)*len(Gap)
}

// Titles returns the titles of the columns that fit.
func (layout Layout) Titles() []string {
	var titles []string
	for index, column := range layout.columns {
		if layout.isShown[index] {
			titles = append(titles, column.Title)
		}
	}
	return titles
}

// Header renders the titles of the columns that fit.
func (layout Layout) Header() string {
	titles := make([]string, len(layout.columns))
	for index, column := range layout.columns {
		titles[index] = column.Title
	}
	return layout.Row(titles)
}

// Row renders the cells of the columns that fit, one cell per column. A cell
// wider than its column is cut with an ellipsis, and a narrower one is padded
// with spaces.
func (layout Layout) Row(row []string) string {
	var fitted []string
	for index, cell := range row {
		if layout.isShown[index] {
			fitted = append(fitted, cells.FitEllipsis(cell, layout.widths[index]))
		}
	}
	return strings.Join(fitted, Gap)
}

// StyledRow renders a row like Row and styles each cell after it is fitted,
// so the padding of a cell takes its style. The gaps take gapStyle.
func (layout Layout) StyledRow(row []string, cellStyles []lipgloss.Style, gapStyle lipgloss.Style) string {
	var fitted []string
	for index, cell := range row {
		if layout.isShown[index] {
			fitted = append(fitted, cellStyles[index].Render(cells.FitEllipsis(cell, layout.widths[index])))
		}
	}
	return strings.Join(fitted, gapStyle.Render(Gap))
}

// nextToDrop returns the index of the shown column with the lowest priority
// that is not required. ok is false when every shown column is required.
func (layout Layout) nextToDrop() (index int, ok bool) {
	dropIndex := -1
	for index, column := range layout.columns {
		isDroppable := layout.isShown[index] && !column.IsRequired && !column.IsFill
		if !isDroppable {
			continue
		}
		if dropIndex == -1 || column.Priority < layout.columns[dropIndex].Priority {
			dropIndex = index
		}
	}
	return dropIndex, dropIndex != -1
}

func (layout Layout) fillIndex() (index int, ok bool) {
	for index, column := range layout.columns {
		if column.IsFill {
			return index, true
		}
	}
	return 0, false
}
