package common

import "strings"

// JoinNames lists names the way a sentence does, with "and" before the last
// one and commas between the others.
func JoinNames(names []string) string {
	if len(names) < 2 {
		return strings.Join(names, "")
	}
	return strings.Join(names[:len(names)-1], ", ") + " and " + names[len(names)-1]
}
