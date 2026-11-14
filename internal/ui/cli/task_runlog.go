package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/IgorBolotnikov/DRUDGE/internal/drudger"
	"github.com/IgorBolotnikov/DRUDGE/internal/task"
	"github.com/IgorBolotnikov/DRUDGE/internal/theme"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/cli/printer"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/style"
)

const (
	notWrittenYetLabel = style.NotWrittenYetLabel
	stderrHeading      = style.StderrHeading
	emptyLogLabel      = "empty"
	logLineIndent      = "  "
)

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
		lines = append(lines, palette.Paint(theme.Stdout, theme.RoleMuted, fmt.Sprintf(style.NoDecoderNote, runSteps.Harness)))
	}
	if runSteps.IsStreamMissing {
		lines = append(lines, notWrittenYetLabel)
	}
	for _, step := range runSteps.Steps {
		lines = append(lines, compactStepLine(palette, step))
	}

	if len(runSteps.Stderr) > 0 {
		lines = append(lines, "", stderrHeading)
		for _, line := range runSteps.Stderr {
			lines = append(lines, palette.Paint(theme.Stdout, theme.RoleError, line))
		}
	}

	out.View(lines)
}

// compactStepLine paints the segments of one Step into the line a compact
// view shows for it. Every line of agent text is indented by the depth of the
// step.
func compactStepLine(palette *theme.Theme, step drudger.Step) string {
	var line strings.Builder
	for _, segment := range style.StepSegments(step) {
		line.WriteString(palette.Paint(theme.Stdout, segment.Role, segment.Text))
	}
	indent := style.StepIndent(step.Depth)
	return indent + strings.ReplaceAll(line.String(), "\n", "\n"+indent)
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
