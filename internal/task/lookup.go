package task

import (
	"errors"
	"fmt"
	"strings"
)

// ErrNoTaskID reports a lookup that was given no id at all.
var ErrNoTaskID = errors.New("task id is required")

// IDMatch names one task an id prefix matched. A repository fills these in
// while searching, so an ambiguous id can be reported without reading every
// task record.
type IDMatch struct {
	ID    TaskID
	Title string
}

// NotFoundError reports that no task carries the given id or id prefix.
func NotFoundError(fullOrPartialID string) error {
	return fmt.Errorf("task %q not found", fullOrPartialID)
}

// AmbiguousIDError reports that an id prefix names several tasks and lists
// them, so a user can pick one and type more characters.
func AmbiguousIDError(fullOrPartialID string, matches []IDMatch) error {
	lines := make([]string, 0, len(matches))
	for _, match := range matches {
		lines = append(lines, fmt.Sprintf("  %s  %s", match.ID, match.Title))
	}
	return fmt.Errorf(
		"task id %q matches %d tasks, add more characters to pick one:\n%s",
		fullOrPartialID, len(matches), strings.Join(lines, "\n"),
	)
}
