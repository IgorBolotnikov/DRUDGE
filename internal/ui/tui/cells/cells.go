// Package cells fits text into a number of terminal cells. A cell is one
// column of the terminal, and a wide character takes two.
package cells

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
)

// Ellipsis ends text that was cut.
const Ellipsis = "…"

// Fit cuts text to width cells and pads it with spaces up to width.
func Fit(text string, width int) string {
	return pad(ansi.Truncate(text, width, ""), width)
}

// FitEllipsis is Fit that ends cut text with an ellipsis.
func FitEllipsis(text string, width int) string {
	return pad(ansi.Truncate(text, width, Ellipsis), width)
}

// BlankLines returns height lines of width spaces.
func BlankLines(width, height int) string {
	if height <= 0 {
		return ""
	}
	line := strings.Repeat(" ", max(width, 0))
	return strings.TrimSuffix(strings.Repeat(line+"\n", height), "\n")
}

func pad(text string, width int) string {
	return text + strings.Repeat(" ", max(width-ansi.StringWidth(text), 0))
}
