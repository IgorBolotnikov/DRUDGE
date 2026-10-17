package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/IgorBolotnikov/DRUDGE/internal/cmd/printer"
	"github.com/IgorBolotnikov/DRUDGE/internal/drudger"
	"github.com/IgorBolotnikov/DRUDGE/internal/task"
	"github.com/IgorBolotnikov/DRUDGE/internal/theme"
)

const (
	notWrittenYetLabel = "not written yet"
	emptyLogLabel      = "empty"
	logLineIndent      = "  "
)

// stderrHeading opens the stderr log of a compact view. It only shows when
// the log holds something.
const stderrHeading = "stderr:"

// noDecoderNote heads a compact view of a run whose harness has no decoder.
// It takes the harness name.
const noDecoderNote = "harness %s has no decoder, showing its raw output"

// depthIndent is how far a compact view indents a step for each level of
// subagent nesting it ran at.
const depthIndent = "  "

// toolColumnWidth is how wide a tool name sits before the summary of a tool
// call.
const toolColumnWidth = 7

// toolErrorIndent puts a tool error under the tool call it belongs to.
const toolErrorIndent = "  "

// Tool names whose summary is a path.
const (
	toolRead  = "Read"
	toolEdit  = "Edit"
	toolWrite = "Write"
)

// pathSummaryTools are the tools whose summary is a path, colored RolePath
// instead of RoleSecondary.
var pathSummaryTools = map[string]bool{
	toolRead:  true,
	toolEdit:  true,
	toolWrite: true,
}

// taskRunlog prints the latest run of a task: a compact view of its steps by
// default, or the raw files the agent wrote with isRaw.
func taskRunlog(taskID task.TaskID, isRaw bool) error {
	deps, err := newCommandDeps()
	if err != nil {
		return err
	}

	if isRaw {
		runLogs, err := deps.drudger.RunLogs(deps.localCfg.ProjectSlug, taskID)
		if err != nil {
			return err
		}
		printRunLogs(deps.out, runLogs)
		return nil
	}

	runSteps, err := deps.drudger.RunSteps(deps.localCfg.ProjectSlug, taskID)
	if err != nil {
		return err
	}
	printRunSteps(deps.out, runSteps)
	return nil
}

// printRunSteps prints a compact view of a run: the task heading, the run
// directory, a note when its harness has no decoder, one line per step and
// the stderr log when it has content.
func printRunSteps(out *printer.Printer, runSteps *drudger.TaskRunSteps) {
	palette := out.Theme()
	lines := []string{
		fmt.Sprintf("Task [%s] %s", runSteps.Task.ID, runSteps.Task.Title),
		palette.Paint(theme.Stdout, theme.RoleMuted, runSteps.RunDir),
	}

	if !runSteps.IsDecoded {
		lines = append(lines, palette.Paint(theme.Stdout, theme.RoleMuted, fmt.Sprintf(noDecoderNote, runSteps.Harness)))
	}
	if runSteps.IsStreamMissing {
		lines = append(lines, notWrittenYetLabel)
	}
	for _, step := range runSteps.Steps {
		lines = append(lines, indentStepLine(compactStepLine(palette, step), step.Depth))
	}

	if len(runSteps.Stderr) > 0 {
		lines = append(lines, "", stderrHeading)
		for _, line := range runSteps.Stderr {
			lines = append(lines, palette.Paint(theme.Stdout, theme.RoleError, line))
		}
	}

	out.View(lines)
}

// compactStepLine renders one Step as the single line a compact view shows
// for it.
func compactStepLine(palette *theme.Theme, step drudger.Step) string {
	switch step.Kind {
	case drudger.StepInit:
		return palette.Paint(theme.Stdout, theme.RoleMuted, step.Summary)
	case drudger.StepUnrecognized:
		return palette.Paint(theme.Stdout, theme.RoleMuted, step.Raw)
	case drudger.StepAgentText:
		return step.Summary
	case drudger.StepToolCall:
		return toolStepLine(palette, step)
	case drudger.StepToolError:
		return toolErrorIndent + palette.Paint(theme.Stdout, theme.RoleError, printer.FailedGlyph+" "+step.Summary)
	case drudger.StepFinalResult:
		return finalResultLine(palette, step)
	default:
		return step.Summary
	}
}

// indentStepLine indents every line of a rendered step by its depth. Agent
// text can span several lines.
func indentStepLine(line string, depth int) string {
	indent := strings.Repeat(depthIndent, depth)
	return indent + strings.ReplaceAll(line, "\n", "\n"+indent)
}

// toolStepLine renders a tool call: the tool name padded to a column followed
// by its summary, or the name alone when there is no summary to pad against.
func toolStepLine(palette *theme.Theme, step drudger.Step) string {
	summaryRole := theme.RoleSecondary
	if pathSummaryTools[step.Tool] {
		summaryRole = theme.RolePath
	}

	if step.Summary == "" {
		return palette.Paint(theme.Stdout, theme.RolePrimary, step.Tool)
	}
	summary := palette.Paint(theme.Stdout, summaryRole, step.Summary)
	tool := palette.Paint(theme.Stdout, theme.RolePrimary, fmt.Sprintf("%-*s", toolColumnWidth, step.Tool))
	return tool + " " + summary
}

// finalResultLine colors the terminal line of a run by its verdict.
func finalResultLine(palette *theme.Theme, step drudger.Step) string {
	role := theme.RoleError
	if strings.HasPrefix(step.Summary, string(drudger.StatusGotShitDone)) {
		role = theme.RoleSuccess
	}
	return palette.Paint(theme.Stdout, role, step.Summary)
}

// printRunLogs prints every log of a run under a heading naming its file.
func printRunLogs(out *printer.Printer, runLogs *drudger.TaskRunLogs) {
	lines := []string{fmt.Sprintf("Task [%s] %s", runLogs.Task.ID, runLogs.Task.Title)}

	for _, runLog := range runLogs.Logs {
		lines = append(lines, "", fmt.Sprintf("%s (%s):", runLog.Name, runLog.Path), "")

		switch {
		case runLog.IsMissing:
			lines = append(lines, notWrittenYetLabel)
		case len(runLog.Lines) == 0:
			lines = append(lines, emptyLogLabel)
		}
		for _, line := range runLog.Lines {
			lines = append(lines, formatLogLine(line))
		}
	}

	out.View(lines)
}

// formatLogLine indents a line that is valid JSON and returns any other line
// unchanged.
func formatLogLine(line string) string {
	var indented bytes.Buffer
	if err := json.Indent(&indented, []byte(strings.TrimSpace(line)), "", logLineIndent); err != nil {
		return line
	}
	return indented.String()
}
