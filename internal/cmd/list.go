package cmd

import (
	"fmt"
	"strings"

	"drudge/internal/common"
	"drudge/internal/task"
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

// column is one column of a listing. A width of zero means the column takes
// whatever room its values need, which only makes sense for the last one.
type column struct {
	Title string
	Width int
}

// printList prints a listing: how many rows there are, a header, a rule under
// it and one line per row. What an empty listing means differs per command, so
// callers say that themselves and only call this once they have rows.
func printList(log *common.Logger, title string, columns []column, rows [][]string) {
	for _, line := range listLines(title, columns, rows) {
		// The line is already formatted and may hold a percent sign.
		log.Info("%s", line)
	}
}

// listLines renders a whole listing, one string per line.
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

// listLine lays values out across the columns. A value too long for its column
// is cut short, and a column the row says nothing about is left blank, so the
// columns line up whatever it is handed.
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

// columnRules draws the line under the header. A column with no width of its
// own is ruled to the width of its title.
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

// fitColumn cuts a value short so it fits its column, marking the cut with an
// ellipsis. A column too narrow to hold the ellipsis is cut without one.
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

// shortTaskID cuts a task id down to the length listings show it at. Ids are
// cut without an ellipsis, because a listing shows the leading characters a
// user types back to name the task.
func shortTaskID(id task.TaskID) string {
	text := string(id)
	if len(text) > task.ShortIDLength {
		return text[:task.ShortIDLength]
	}
	return text
}
