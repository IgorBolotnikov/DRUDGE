package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/IgorBolotnikov/DRUDGE/internal/cmd/printer"
	"github.com/IgorBolotnikov/DRUDGE/internal/drudger"
	"github.com/IgorBolotnikov/DRUDGE/internal/task"
)

const (
	notWrittenYetLabel = "not written yet"
	emptyLogLabel      = "empty"
	logLineIndent      = "  "
)

// taskRunlog prints the logs of the latest run of a task.
func taskRunlog(args []string) error {
	taskID := task.TaskID(args[0])

	deps, err := newCommandDeps()
	if err != nil {
		return err
	}

	runLogs, err := deps.drudger.RunLogs(deps.localCfg.ProjectSlug, taskID)
	if err != nil {
		return err
	}

	printRunLogs(deps.out, runLogs)
	return nil
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
