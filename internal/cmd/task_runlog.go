package cmd

import (
	"bytes"
	"encoding/json"
	"strings"

	"github.com/IgorBolotnikov/DRUDGE/internal/common"
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

	printRunLogs(deps.log, runLogs)
	return nil
}

// printRunLogs prints every log of a run under a heading naming its file.
func printRunLogs(log *common.Logger, runLogs *drudger.TaskRunLogs) {
	log.Info("Task [%s] %s", runLogs.Task.ID, runLogs.Task.Title)

	for _, runLog := range runLogs.Logs {
		log.Info("")
		log.Info("%s (%s):", runLog.Name, runLog.Path)
		log.Info("")

		switch {
		case runLog.IsMissing:
			log.Info(notWrittenYetLabel)
		case len(runLog.Lines) == 0:
			log.Info(emptyLogLabel)
		}
		for _, line := range runLog.Lines {
			// A log line may hold a percent sign.
			log.Info("%s", formatLogLine(line))
		}
	}
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
