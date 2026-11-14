package drudger

import (
	"bytes"
	"fmt"
	"path/filepath"
	"slices"
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

// RunCursor marks how far a reader got through the latest run of a task. Its
// zero value reads a run from the start.
type RunCursor struct {
	runID        string
	streamOffset int64
	stderrOffset int64
	// streamTail and stderrTail hold the bytes after the last newline read
	// from each file, which do not make a complete line yet.
	streamTail []byte
	stderrTail []byte
}

func (cursor RunCursor) isZero() bool {
	return cursor.runID == "" && cursor.streamOffset == 0 && cursor.stderrOffset == 0
}

// RunStepsRead is what one read of the latest run of a task found since the
// cursor it started from.
type RunStepsRead struct {
	Task   *task.Task
	RunDir string
	// HasRun is false for a task with no run. Every field below is then
	// empty and Next is the zero cursor.
	HasRun bool
	// IsReset is true when the task was run again since the cursor. The read
	// then starts over, and Steps and Stderr hold everything of the new run.
	IsReset bool
	// Harness is the harness the steps were read for. IsDecoded is false when
	// DRUDGE has no decoder for it, and each Step then holds one raw line.
	Harness   Harness
	IsDecoded bool
	// Steps holds the steps of the event stream lines completed since the
	// cursor. IsStreamMissing is true while the agent has not written the
	// event stream yet.
	Steps           []Step
	IsStreamMissing bool
	// Stderr holds the stderr log lines completed since the cursor.
	Stderr []string
	// Next is the cursor the following read starts from.
	Next RunCursor
}

// RunStepsFrom decodes the event stream lines the latest run of a task wrote
// after a cursor into the harness-neutral Steps a compact view renders. A
// line without its newline yet stays in the cursor until the newline arrives.
// It uses the decoder of the harness the run recorded, or of the configured
// harness for a run that recorded none. A harness with no decoder gives one
// unrecognized Step per line.
func (service *DrudgerService) RunStepsFrom(projectSlug string, requestedID task.TaskID, cursor RunCursor) (*RunStepsRead, error) {
	tracked, err := service.tasks.GetTask(projectSlug, requestedID)
	if err != nil {
		return nil, err
	}

	layout, err := service.layout()
	if err != nil {
		return nil, err
	}
	read := &RunStepsRead{Task: tracked, RunDir: layout.RunDir(tracked.ID)}

	hasRun, err := service.runs.HasRun(tracked.ID)
	if err != nil {
		return nil, err
	}
	if !hasRun {
		return read, nil
	}
	read.HasRun = true

	runID, err := service.runs.ReadRunID(tracked.ID)
	if err != nil {
		return nil, fmt.Errorf("could not read the run ID of task %s: %w", tracked.ID, err)
	}
	if runID != cursor.runID {
		read.IsReset = !cursor.isZero()
		cursor = RunCursor{runID: runID}
	}

	harness, err := service.runReader().harnessOf(tracked.ID)
	if err != nil {
		return nil, err
	}
	decoder, isDecoded := service.streams[harness]
	read.Harness = harness
	read.IsDecoded = isDecoded

	stream, isStreamPresent, err := service.runs.ReadStreamFrom(tracked.ID, cursor.streamOffset)
	if err != nil {
		return nil, fmt.Errorf("could not read the logs of task %s: %w", tracked.ID, err)
	}
	stderr, _, err := service.runs.ReadStderrFrom(tracked.ID, cursor.stderrOffset)
	if err != nil {
		return nil, fmt.Errorf("could not read the logs of task %s: %w", tracked.ID, err)
	}

	streamLines, streamTail := completeLines(cursor.streamTail, stream)
	stderrLines, stderrTail := completeLines(cursor.stderrTail, stderr)
	read.Steps = decodeSteps(streamLines, decoder, isDecoded)
	read.IsStreamMissing = !isStreamPresent
	read.Stderr = stderrLines
	read.Next = RunCursor{
		runID:        runID,
		streamOffset: cursor.streamOffset + int64(len(stream)),
		stderrOffset: cursor.stderrOffset + int64(len(stderr)),
		streamTail:   streamTail,
		stderrTail:   stderrTail,
	}
	return read, nil
}

// completeLines splits the bytes left over from an earlier read and the bytes
// read since into the complete lines they hold and the bytes after the last
// newline.
func completeLines(tail []byte, content []byte) (lines []string, rest []byte) {
	joined := slices.Concat(tail, content)
	lastNewline := bytes.LastIndexByte(joined, '\n')
	if lastNewline < 0 {
		return nil, joined
	}
	return logLines(joined[:lastNewline+1]), joined[lastNewline+1:]
}

// decodeSteps turns event stream lines into steps. Without a decoder each line
// becomes one unrecognized step.
func decodeSteps(lines []string, decoder StreamReader, isDecoded bool) []Step {
	var steps []Step
	for _, line := range lines {
		if !isDecoded {
			steps = append(steps, Step{Kind: StepUnrecognized, Raw: line})
			continue
		}
		steps = append(steps, decoder.Decode(line)...)
	}
	return steps
}

// RunSteps decodes the whole event stream and stderr log of the latest run of
// a task. A last line without its newline counts as complete. It refuses a
// task with no run directory.
func (service *DrudgerService) RunSteps(projectSlug string, requestedID task.TaskID) (*TaskRunSteps, error) {
	read, err := service.RunStepsFrom(projectSlug, requestedID, RunCursor{})
	if err != nil {
		return nil, err
	}
	if !read.HasRun {
		return nil, fmt.Errorf("task %s is %q and has no run to show the logs of, run it first", read.Task.ID, read.Task.Status)
	}

	decoder := service.streams[read.Harness]
	steps := slices.Concat(read.Steps, decodeSteps(logLines(read.Next.streamTail), decoder, read.IsDecoded))
	stderr := slices.Concat(read.Stderr, logLines(read.Next.stderrTail))
	return &TaskRunSteps{
		Task:            read.Task,
		RunDir:          read.RunDir,
		Harness:         read.Harness,
		IsDecoded:       read.IsDecoded,
		Steps:           steps,
		IsStreamMissing: read.IsStreamMissing,
		Stderr:          stderr,
	}, nil
}
