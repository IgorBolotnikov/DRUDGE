package style

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/IgorBolotnikov/DRUDGE/internal/drudger"
	"github.com/IgorBolotnikov/DRUDGE/internal/theme"
)

// Labels of a compact view of a run.
const (
	// StderrHeading opens the stderr log. It only shows when the log holds
	// something.
	StderrHeading = "stderr:"
	// NotWrittenYetLabel stands in for a log the agent has not written yet.
	NotWrittenYetLabel = "not written yet"
	// NoDecoderNote heads the view of a run whose harness has no decoder. It
	// takes the harness name.
	NoDecoderNote = "harness %s has no decoder, showing its raw output"
)

// FailedGlyph starts a line about work that failed.
const FailedGlyph = "✗"

// DepthIndent is how far a step is indented for each level of subagent
// nesting it ran at.
const DepthIndent = "  "

// ToolColumnWidth is how wide a tool name sits before the summary of a tool
// call.
const ToolColumnWidth = 7

// toolSummarySeparator sits between the tool column and the summary of a
// tool call.
const toolSummarySeparator = " "

// ToolErrorIndent puts a tool error under the tool call it belongs to.
const ToolErrorIndent = "  "

// Tool names whose summary is a path.
const (
	toolRead  = "Read"
	toolEdit  = "Edit"
	toolWrite = "Write"
)

// PathSummaryTools are the tools whose summary is a path, painted RolePath
// instead of RoleSecondary.
var PathSummaryTools = map[string]bool{
	toolRead:  true,
	toolEdit:  true,
	toolWrite: true,
}

// StepRoles maps a step kind to the theme role it paints in. Agent text is
// left out and paints plain. A tool call and a final result take their roles
// from their content.
var StepRoles = map[drudger.StepKind]string{
	drudger.StepInit:         theme.RoleMuted,
	drudger.StepUnrecognized: theme.RoleMuted,
	drudger.StepToolError:    theme.RoleError,
}

// Segment is a piece of a rendered step with the theme role it paints in. An
// empty role paints plain.
type Segment struct {
	Text string
	Role string
}

// StepSegments breaks a step into the segments of its compact view. It leaves
// out the depth indent, and agent text keeps its line breaks.
func StepSegments(step drudger.Step) []Segment {
	switch step.Kind {
	case drudger.StepUnrecognized:
		return []Segment{{Text: step.Raw, Role: StepRoles[step.Kind]}}
	case drudger.StepToolCall:
		return toolCallSegments(step)
	case drudger.StepToolError:
		return []Segment{
			{Text: ToolErrorIndent},
			{Text: FailedGlyph + " " + step.Summary, Role: StepRoles[step.Kind]},
		}
	case drudger.StepFinalResult:
		return []Segment{{Text: step.Summary, Role: FinalResultRole(step.Summary)}}
	default:
		return []Segment{{Text: step.Summary, Role: StepRoles[step.Kind]}}
	}
}

// toolCallSegments renders a tool call as the tool name padded to the tool
// column, a space and the summary. A call with no summary is the name alone.
func toolCallSegments(step drudger.Step) []Segment {
	if step.Summary == "" {
		return []Segment{{Text: step.Tool, Role: theme.RolePrimary}}
	}

	summaryRole := theme.RoleSecondary
	if PathSummaryTools[step.Tool] {
		summaryRole = theme.RolePath
	}
	return []Segment{
		{Text: paddedToolName(step.Tool), Role: theme.RolePrimary},
		{Text: toolSummarySeparator},
		{Text: step.Summary, Role: summaryRole},
	}
}

func paddedToolName(tool string) string {
	return fmt.Sprintf("%-*s", ToolColumnWidth, tool)
}

// FinalResultRole is the role of the final result of a run: success when the
// run got shit done and error otherwise.
func FinalResultRole(summary string) string {
	if strings.HasPrefix(summary, string(drudger.StatusGotShitDone)) {
		return theme.RoleSuccess
	}
	return theme.RoleError
}

// StepIndent is the indent of a step that ran at depth.
func StepIndent(depth int) string {
	return strings.Repeat(DepthIndent, depth)
}

// StepHangingIndent is the indent of the continuation lines of a wrapped
// step, depth indent included. It puts the continuation lines of a tool call
// under the start of its summary and those of a tool error under the start of
// its message.
func StepHangingIndent(step drudger.Step) string {
	indent := StepIndent(step.Depth)
	switch step.Kind {
	case drudger.StepToolCall:
		return indent + blankOf(paddedToolName(step.Tool)+toolSummarySeparator)
	case drudger.StepToolError:
		return indent + ToolErrorIndent + blankOf(FailedGlyph+" ")
	default:
		return indent
	}
}

// blankOf is as many spaces as text has characters.
func blankOf(text string) string {
	return strings.Repeat(" ", utf8.RuneCountInString(text))
}
