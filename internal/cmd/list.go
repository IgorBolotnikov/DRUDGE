package cmd

import (
	"fmt"
	"strings"

	"github.com/IgorBolotnikov/DRUDGE/internal/common"
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
type column struct {
	Title string
	Width int
}

// printList prints a listing: how many rows there are, a header, a rule under
// it and one line per row.
func printList(log *common.Logger, title string, columns []column, rows [][]string) {
	for _, line := range listLines(title, columns, rows) {
		// The line is already formatted and may hold a percent sign.
		log.Info("%s", line)
	}
}

func listLines(title string, columns []column, rows [][]string) []string {
	lines := make([]string, 0, len(rows)+3)
	lines = append(lines, fmt.Sprintf("%s (%d):", title, len(rows)))
	lines = append(lines, listLine(columns, columnTitles(columns)))
	lines = append(lines, listLine(columns, columnRules(columns)))
	for _, row := range rows {
		lines = append(lines, listLine(columns, row))
	}
	return lines
}

func listLine(columns []column, values []string) string {
	cells := make([]string, 0, len(columns))
	for index, col := range columns {
		var value string
		if index < len(values) {
			value = values[index]
		}
		if col.Width > 0 {
			value = fmt.Sprintf("%-*s", col.Width, fitColumn(value, col.Width))
		}
		cells = append(cells, value)
	}
	return listIndent + strings.TrimRight(strings.Join(cells, listGap), " ")
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
