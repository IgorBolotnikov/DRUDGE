package drudger

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
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

	runDir := layout.RunDir(tracked.ID)
	hasRunDir, err := common.Exists(runDir)
	if err != nil {
		return nil, err
	}
	if !hasRunDir {
		return nil, fmt.Errorf("task %s is %q and has no run to show the logs of, run it first", tracked.ID, tracked.Status)
	}

	logs := []RunLog{}
	for _, path := range []string{common.RunStreamPath(runDir), common.RunStderrPath(runDir)} {
		log, err := readRunLog(path)
		if err != nil {
			return nil, err
		}
		logs = append(logs, log)
	}
	return &TaskRunLogs{Task: tracked, Logs: logs}, nil
}

// readRunLog reads one log file of a run directory. It reports a file the
// agent has not written yet as missing.
func readRunLog(path string) (RunLog, error) {
	log := RunLog{Name: filepath.Base(path), Path: path}

	content, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		log.IsMissing = true
		return log, nil
	}
	if err != nil {
		return RunLog{}, fmt.Errorf("could not read the run log %s: %w", path, err)
	}

	if len(content) > 0 {
		log.Lines = strings.Split(strings.TrimSuffix(string(content), "\n"), "\n")
	}
	return log, nil
}
