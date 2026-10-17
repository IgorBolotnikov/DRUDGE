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
	return RunLog{Name: filepath.Base(path), Path: path, Lines: logLines(content), IsMissing: !isPresent}
}

// logLines splits the content of a run log file into lines, dropping its
// trailing newline. Content with nothing in it has no lines.
func logLines(content []byte) []string {
	if len(content) == 0 {
		return nil
	}
	return strings.Split(strings.TrimSuffix(string(content), "\n"), "\n")
}

// TaskRunSteps pairs a task with the steps decoded from its latest run and the
// stderr log beside them.
type TaskRunSteps struct {
	Task   *task.Task
	RunDir string
	// Harness is the harness the steps were read for. IsDecoded is false when
	// DRUDGE has no decoder for it, and each Step then holds one raw line.
	Harness   Harness
	IsDecoded bool
	// Steps is empty and IsStreamMissing is true while the agent has not
	// written the event stream yet.
	Steps           []Step
	IsStreamMissing bool
	// Stderr is empty when the run has no stderr log, or an empty one.
	Stderr []string
}

// RunSteps decodes the event stream of the latest run of a task into the
// harness-neutral Steps a compact view renders. It uses the decoder of the
// harness the run recorded, or of the configured harness for a run that
// recorded none. A harness with no decoder gives one unrecognized Step per
// line. It refuses a task with no run directory.
func (service *DrudgerService) RunSteps(projectSlug string, requestedID task.TaskID) (*TaskRunSteps, error) {
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

	harness, isRecorded, err := service.runs.ReadHarness(tracked.ID)
	if err != nil {
		return nil, fmt.Errorf("could not read the harness of the run of task %s: %w", tracked.ID, err)
	}
	if !isRecorded {
		harness = service.settings.Harness
	}
	decoder, isDecoded := service.decoders[harness]

	runDir := layout.RunDir(tracked.ID)
	stream, isStreamPresent, err := service.runs.ReadStream(tracked.ID)
	if err != nil {
		return nil, fmt.Errorf("could not read the logs of task %s: %w", tracked.ID, err)
	}
	stderr, _, err := service.runs.ReadStderr(tracked.ID)
	if err != nil {
		return nil, fmt.Errorf("could not read the logs of task %s: %w", tracked.ID, err)
	}

	var steps []Step
	for _, line := range logLines(stream) {
		if !isDecoded {
			steps = append(steps, Step{Kind: StepUnrecognized, Raw: line})
			continue
		}
		steps = append(steps, decoder.Decode(line)...)
	}

	return &TaskRunSteps{
		Task:            tracked,
		RunDir:          runDir,
		Harness:         harness,
		IsDecoded:       isDecoded,
		Steps:           steps,
		IsStreamMissing: !isStreamPresent,
		Stderr:          logLines(stderr),
	}, nil
}
