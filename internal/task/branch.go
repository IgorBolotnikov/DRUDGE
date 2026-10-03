package task

import (
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"unicode"
)

// Placeholders a branch name format may carry.
const (
	BranchPlaceholderShortID = "{{taskShortID}}"
	BranchPlaceholderSlug    = "{{taskSlug}}"
)

// branchPlaceholders are the placeholders BranchName fills in.
var branchPlaceholders = []string{BranchPlaceholderShortID, BranchPlaceholderSlug}

// branchPlaceholderPattern matches anything a format writes as a placeholder.
var branchPlaceholderPattern = regexp.MustCompile(`\{\{[^{}]*\}\}`)

// branchSlugSeparator joins the words of a slug and parts of a branch name.
const branchSlugSeparator = "-"

// branchSegmentSeparator splits a branch name into the segments git checks one
// by one.
const branchSegmentSeparator = "/"

// branchSegmentEdges are trimmed from both ends of every segment of a branch
// name.
const branchSegmentEdges = "-_./"

// DefaultBranchFormat names the branch of a task when nothing configures one.
// TODO: figure out per-task prefixes that come from task types.
const DefaultBranchFormat = "drudge/" + BranchPlaceholderShortID + branchSlugSeparator + BranchPlaceholderSlug

// branchSlugLength caps how much of a task title a branch name carries.
const branchSlugLength = 40

// sampleBranchTask fills a format in when ValidateBranchFormat checks the name
// it gives.
var sampleBranchTask = &Task{ID: "3f9a1c2e-0000-4000-8000-000000000000", Title: "Add retry to uploader"}

// BranchName fills the placeholders of a branch name format in with a task.
// It then collapses repeated separators, trims separators from both ends of
// every segment and drops empty segments.
func BranchName(format string, task *Task) string {
	filled := strings.NewReplacer(
		BranchPlaceholderShortID, ShortID(task.ID),
		BranchPlaceholderSlug, branchSlug(task.Title),
	).Replace(format)

	segments := []string{}
	for segment := range strings.SplitSeq(filled, branchSegmentSeparator) {
		if trimmed := strings.Trim(segment, branchSegmentEdges); trimmed != "" {
			segments = append(segments, trimmed)
		}
	}
	return strings.Join(segments, branchSegmentSeparator)
}

// ValidateBranchFormat rejects a branch name format that carries neither the
// short id nor the slug placeholder, one with a placeholder BranchName does not
// fill in, and one that gives a name git refuses for a sample task.
func ValidateBranchFormat(format string) error {
	if !strings.Contains(format, BranchPlaceholderShortID) && !strings.Contains(format, BranchPlaceholderSlug) {
		return fmt.Errorf("it needs %s or %s", BranchPlaceholderShortID, BranchPlaceholderSlug)
	}

	for _, placeholder := range branchPlaceholderPattern.FindAllString(format, -1) {
		if !slices.Contains(branchPlaceholders, placeholder) {
			return fmt.Errorf("it has the unknown placeholder %s, the known ones are %s", placeholder, strings.Join(branchPlaceholders, ", "))
		}
	}

	sample := BranchName(format, sampleBranchTask)
	if err := ValidateBranchName(sample); err != nil {
		return fmt.Errorf("it gives the branch name %q for a sample task: %w", sample, err)
	}
	return nil
}

// branchLockSuffix ends the lock files git keeps beside refs, so no segment of
// a ref may end with it.
const branchLockSuffix = ".lock"

// branchForbiddenText is text git refuses anywhere in a ref.
var branchForbiddenText = []string{" ", "..", "~", "^", ":", "?", "*", "[", "\\", "@{"}

// ValidateBranchName rejects a branch name that breaks the rules git has for a
// ref.
func ValidateBranchName(name string) error {
	switch {
	case name == "":
		return errors.New("a branch name cannot be empty")
	case name == "@":
		return errors.New("a branch name cannot be @")
	case strings.HasSuffix(name, ".") || strings.HasSuffix(name, branchSegmentSeparator):
		return errors.New("a branch name cannot end with . or /")
	}

	for _, forbidden := range branchForbiddenText {
		if strings.Contains(name, forbidden) {
			return fmt.Errorf("a branch name cannot contain %q", forbidden)
		}
	}
	if strings.ContainsFunc(name, unicode.IsControl) {
		return errors.New("a branch name cannot contain control characters")
	}

	for segment := range strings.SplitSeq(name, branchSegmentSeparator) {
		switch {
		case segment == "":
			return errors.New("a branch name cannot start with / or contain //")
		case strings.HasPrefix(segment, "."):
			return fmt.Errorf("a segment of a branch name cannot start with ., %q does", segment)
		case strings.HasSuffix(segment, branchLockSuffix):
			return fmt.Errorf("a segment of a branch name cannot end with %s, %q does", branchLockSuffix, segment)
		}
	}
	return nil
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
