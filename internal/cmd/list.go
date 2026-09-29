package cmd

import (
	"flag"
	"fmt"
	"strings"

	"github.com/IgorBolotnikov/DRUDGE/internal/common"
	"github.com/IgorBolotnikov/DRUDGE/internal/theme"
)

const (
	// listIndent starts every line of a listing.
	listIndent = "  "
	// listGap separates two columns.
	listGap = "  "
	// listRule is the character the line under the header is drawn with.
	listRule = "-"
	// listEllipsis marks a value that was cut short to fit its column.
	listEllipsis = "..."
)

// labelledLine formats one line of a report. width lines the values up and
// counts the colon that follows every label.
func labelledLine(label string, width int, value string) string {
	return fmt.Sprintf("%s%-*s %s", listIndent, width, label+":", value)
}

// column is one column of a listing. A width of zero means the column takes
// whatever room its values need (which only makes sense for the last one).
// Color, when set, wraps the text of every row cell in the column. The header
// and the rule stay plain.
type column struct {
	Title string
	Width int
	Color func(text string) string
}

// printList prints a listing: the total count of rows on every page, a header,
// a rule under it and one line per row.
func printList(log *common.Logger, title string, total int, columns []column, rows [][]string) {
	printColoredList(log, title, total, columns, rows, nil)
}

// printColoredList prints a listing like printList. A row with a color in
// rowColors prints its whole line in that color, and the colors of the
// columns skip it.
func printColoredList(log *common.Logger, title string, total int, columns []column, rows [][]string, rowColors []func(text string) string) {
	for _, line := range listLines(title, total, columns, rows, rowColors) {
		// The line is already formatted and may hold a percent sign.
		log.Info("%s", line)
	}
}

func listLines(title string, total int, columns []column, rows [][]string, rowColors []func(text string) string) []string {
	lines := make([]string, 0, len(rows)+3)
	lines = append(lines, fmt.Sprintf("%s (%d):", title, total))
	lines = append(lines, listLine(columns, columnTitles(columns), false))
	lines = append(lines, listLine(columns, columnRules(columns), false))
	for index, row := range rows {
		if index < len(rowColors) && rowColors[index] != nil {
			lines = append(lines, colorCell(listLine(columns, row, false), rowColors[index]))
			continue
		}
		lines = append(lines, listLine(columns, row, true))
	}
	return lines
}

func listLine(columns []column, values []string, shouldColor bool) string {
	cells := make([]string, 0, len(columns))
	for index, col := range columns {
		var value string
		if index < len(values) {
			value = values[index]
		}
		if col.Width > 0 {
			value = fmt.Sprintf("%-*s", col.Width, fitColumn(value, col.Width))
		}
		if shouldColor && col.Color != nil {
			value = colorCell(value, col.Color)
		}
		cells = append(cells, value)
	}
	return listIndent + strings.TrimRight(strings.Join(cells, listGap), " ")
}

// colorCell colors the text of a cell that is already fitted and padded. The
// leading indent and the trailing padding stay plain. A blank cell stays
// plain, so listLine still trims it off the end of the line.
func colorCell(cell string, color func(text string) string) string {
	text := strings.TrimLeft(cell, " ")
	indent := cell[:len(cell)-len(text)]
	text = strings.TrimRight(text, " ")
	if text == "" {
		return cell
	}
	padding := cell[len(indent)+len(text):]
	return indent + color(text) + padding
}

func columnTitles(columns []column) []string {
	titles := make([]string, 0, len(columns))
	for _, col := range columns {
		titles = append(titles, col.Title)
	}
	return titles
}

// columnRules draws the line under the header.
func columnRules(columns []column) []string {
	rules := make([]string, 0, len(columns))
	for _, col := range columns {
		width := col.Width
		if width == 0 {
			width = len([]rune(col.Title))
		}
		rules = append(rules, strings.Repeat(listRule, width))
	}
	return rules
}

// Flags that page a listing.
const (
	pageFlagName      = "page"
	pageFlagShortName = "p"
	pageSizeFlagName  = "page-size"
)

// pageFlags holds which page of a listing to show and how big a page is.
type pageFlags struct {
	number int
	size   optionalInt
}

// declare declares the page flags. sizeSource names the config the page size
// comes from when the size flag is left out.
func (flags *pageFlags) declare(fs *flag.FlagSet, sizeSource string) {
	fs.IntVar(&flags.number, pageFlagName, 1, "The `number` of the page to show")
	alias(fs, pageFlagShortName, pageFlagName)
	fs.Var(&flags.size, pageSizeFlagName, "How many `items` one page shows, 0 shows them all, "+sizeSource+" when left out")
}

// pageSize returns the size given with the size flag, or fallback when the
// flag was left out. It refuses a negative size.
func (flags *pageFlags) pageSize(fallback int) (int, error) {
	if flags.size.value == nil {
		return fallback, nil
	}
	if *flags.size.value < 0 {
		return 0, fmt.Errorf("--%s must be 0 or more, got %d", pageSizeFlagName, *flags.size.value)
	}
	return *flags.size.value, nil
}

// printPageFooter prints which page of how many a listing shows, in the muted
// color of the theme. Every page but the last names the flag that shows the
// next one. A listing of one page prints nothing.
func printPageFooter(log *common.Logger, number int, totalPages int) {
	if totalPages <= 1 {
		return
	}
	footer := fmt.Sprintf("Page %d of %d", number, totalPages)
	if number < totalPages {
		footer += fmt.Sprintf(", see the next one with --%s %d", pageFlagName, number+1)
	}

	palette, err := theme.Load("")
	if err != nil {
		log.Error("cannot color the page footer: %v", err)
	} else {
		footer = palette.Color(theme.RoleMuted) + footer + palette.Reset()
	}
	log.Info("%s", footer)
}

// fitColumn cuts a value short so it fits its column.
func fitColumn(value string, width int) string {
	runes := []rune(value)
	if len(runes) <= width {
		return value
	}
	if width <= len(listEllipsis) {
		return string(runes[:width])
	}
	return string(runes[:width-len(listEllipsis)]) + listEllipsis
}
