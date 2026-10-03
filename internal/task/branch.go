package task

import (
	"strings"
	"unicode"
)

// Placeholders a branch name format may carry.
const (
	BranchPlaceholderShortID = "{{taskShortID}}"
	BranchPlaceholderSlug    = "{{taskSlug}}"
)

// branchSlugSeparator joins the words of a slug and parts of a branch name.
const branchSlugSeparator = "-"

// DefaultBranchFormat names the branch of a task when nothing configures one.
// TODO: figure out per-task prefixes that come from task types.
const DefaultBranchFormat = "drudge/" + BranchPlaceholderShortID + branchSlugSeparator + BranchPlaceholderSlug

// branchSlugLength caps how much of a task title a branch name carries.
const branchSlugLength = 40

// BranchName fills the placeholders of a branch name format in with a task.
// A title that folds to an empty slug drops the slug placeholder together with
// the separator next to it.
func BranchName(format string, task *Task) string {
	slug := branchSlug(task.Title)
	if slug == "" {
		format = strings.NewReplacer(
			branchSlugSeparator+BranchPlaceholderSlug, "",
			BranchPlaceholderSlug+branchSlugSeparator, "",
		).Replace(format)
	}

	return strings.NewReplacer(
		BranchPlaceholderShortID, ShortID(task.ID),
		BranchPlaceholderSlug, slug,
	).Replace(format)
}

// branchSlug folds a task title into the part of a branch name that comes from
// it. Everything git does not accept in a ref becomes a separator, and a title
// longer than the slug length is cut at a word.
func branchSlug(title string) string {
	words := strings.FieldsFunc(strings.ToLower(title), func(letter rune) bool {
		return !unicode.IsLetter(letter) && !unicode.IsDigit(letter)
	})

	slug := []rune(strings.Join(words, branchSlugSeparator))
	if len(slug) <= branchSlugLength {
		return string(slug)
	}

	cut := string(slug[:branchSlugLength])
	if lastWord := strings.LastIndex(cut, branchSlugSeparator); lastWord > 0 {
		return cut[:lastWord]
	}
	return cut
}
