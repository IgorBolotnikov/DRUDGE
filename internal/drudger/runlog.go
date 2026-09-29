package drudger

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/IgorBolotnikov/DRUDGE/internal/common"
	"github.com/IgorBolotnikov/DRUDGE/internal/task"
)

// RunLog is one log file of a run directory.
type RunLog struct {
	Name      string
	Path      string
	Lines     []string // the content without its trailing newline, empty for an empty file
	IsMissing bool     // the agent has not written the file yet
}

// TaskRunLogs pairs a task with the log files of its latest run.
type TaskRunLogs struct {
	Task *task.Task
	Logs []RunLog
}

// RunLogs reads the event stream and the stderr log of the latest run of a
// task, in that order. It refuses a task with no run directory.
func (service *DrudgerService) RunLogs(projectSlug string, requestedID task.TaskID) (*TaskRunLogs, error) {
	tracked, err := service.tasks.GetTask(projectSlug, requestedID)
	if err != nil {
		return nil, err
	}

	layout, err := service.layout()
	if err != nil {
		return nil, err
	}

	hasRun, err := service.runs.HasRun(tracked.ID)
	if err != nil {
		return nil, err
	}
	if !hasRun {
		return nil, fmt.Errorf("task %s is %q and has no run to show the logs of, run it first", tracked.ID, tracked.Status)
	}

	runDir := layout.RunDir(tracked.ID)
	stream, isStreamPresent, err := service.runs.ReadStream(tracked.ID)
	if err != nil {
		return nil, fmt.Errorf("could not read the logs of task %s: %w", tracked.ID, err)
	}
	stderr, isStderrPresent, err := service.runs.ReadStderr(tracked.ID)
	if err != nil {
		return nil, fmt.Errorf("could not read the logs of task %s: %w", tracked.ID, err)
	}

	logs := []RunLog{
		runLogOf(common.RunStreamPath(runDir), stream, isStreamPresent),
		runLogOf(common.RunStderrPath(runDir), stderr, isStderrPresent),
	}
	return &TaskRunLogs{Task: tracked, Logs: logs}, nil
}

// runLogOf splits the content of one log file of a run directory into lines.
// It reports a file the agent has not written yet as missing.
func runLogOf(path string, content []byte, isPresent bool) RunLog {
	log := RunLog{Name: filepath.Base(path), Path: path, IsMissing: !isPresent}
	if len(content) > 0 {
		log.Lines = strings.Split(strings.TrimSuffix(string(content), "\n"), "\n")
	}
	return log
}
