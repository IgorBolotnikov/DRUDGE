package common

import (
	"regexp"
	"strings"
)

// JoinNames lists names the way a sentence does, with "and" before the last
// one and commas between the others.
func JoinNames(names []string) string {
	if len(names) < 2 {
		return strings.Join(names, "")
	}
	return strings.Join(names[:len(names)-1], ", ") + " and " + names[len(names)-1]
}

// ansiEscapePattern matches a CSI sequence such as a color or a cursor move,
// an OSC sequence such as a window title or a link, and a two byte escape.
var ansiEscapePattern = regexp.MustCompile(`\x1b\[[0-?]*[ -/]*[@-~]|\x1b\][^\x07\x1b]*(?:\x07|\x1b\\)|\x1b[@-Z\\-_]`)

// StripANSI removes the ANSI escape sequences from text.
func StripANSI(text string) string {
	return ansiEscapePattern.ReplaceAllString(text, "")
}
