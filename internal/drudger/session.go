package drudger

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strconv"
	"strings"
	"time"

	"drudge/internal/common"
	"drudge/internal/task"
)

// SessionStatus is how the work of one Session is going. It answers a
// different question from the Task's own status and from the Drudger's health,
// hence its own type.
type SessionStatus string

const (
	// StatusWorking means the agent is still running and still doing something.
	StatusWorking SessionStatus = "working"
	// StatusNeedsBabysitting means the agent is still running but has gone
	// quiet for long enough to be worth a look.
	StatusNeedsBabysitting SessionStatus = "needs babysitting"
	// StatusFuckedUp means the agent stopped without doing or finishing the work.
	StatusFuckedUp SessionStatus = "fucked up"
	// StatusGotShitDone means the agent finished the work it was given.
	StatusGotShitDone SessionStatus = "got shit done"
)

// sessionStaleAfter is how long a running Session may write nothing before
// drudge says it needs babysitting. Note that a single tool call can take
// minutes on its own.
// TODO: move it to the config
const sessionStaleAfter = 5 * time.Minute

// errNoRunDirectory reports a task that has no run directory,
// and therefore no Session to check.
var errNoRunDirectory = errors.New("no run directory")

// SessionReport is what the run directory of a task says about the last
// Session that worked on it.
type SessionReport struct {
	Status    SessionStatus
	RunDir    string         // where the files of the run live
	SessionID string         // resumable agent session, empty until the agent reports it
	LastWrite time.Time      // when the Session last produced output
	ExitCode  int            // what the agent exited with, meaningful once the Session has finished
	Result    *SessionResult // what the agent reported at the end, nil until it gets there
}

// SessionResult is what an agent reported on the terminal event of its run.
type SessionResult struct {
	IsError  bool          // whether the agent flagged the run as an error
	Subtype  string        // how the run ended, "success" when the agent got through it
	NumTurns int           // how many turns the agent took
	Duration time.Duration // how long the agent worked
	CostUSD  float64       // what the run cost
	Text     string        // the last thing the agent said
}

// Finished reports whether the Session has stopped, whatever it left behind.
func (report SessionReport) Finished() bool {
	return report.Status == StatusFuckedUp || report.Status == StatusGotShitDone
}

// TaskSession pairs a task with what its run directory says about the last
// Session that worked on it.
type TaskSession struct {
	Task   *task.Task
	Report SessionReport
}

// SessionStatus tells how the last Session of a task is going.
func (service *DrudgerService) SessionStatus(projectSlug string, requestedID task.TaskID) (*TaskSession, error) {
	tracked, err := service.tasks.GetTask(projectSlug, requestedID)
	if err != nil {
		return nil, err
	}

	workspace, err := common.WorkDir()
	if err != nil {
		return nil, fmt.Errorf("could not work out where task %s runs: %w", tracked.ID, err)
	}

	report, err := readSessionReport(common.RunDir(workspace, string(tracked.ID)), time.Now().UTC())
	if errors.Is(err, errNoRunDirectory) {
		return nil, fmt.Errorf("task %s is %q and has no run to report on, run it first", tracked.ID, tracked.Status)
	}
	if err != nil {
		return nil, err
	}

	if err := service.recordOutcome(projectSlug, tracked, report); err != nil {
		return nil, err
	}

	return &TaskSession{Task: tracked, Report: report}, nil
}

// recordOutcome writes what a finished Session left behind onto its task, so
// the task record says what happened without anyone reading the run directory.
//
// A Session that is still working leaves its task alone, and a task already
// carrying a finish time is left as it is, so checking twice records once.
func (service *DrudgerService) recordOutcome(projectSlug string, tracked *task.Task, report SessionReport) error {
	if !report.Finished() || !tracked.FinishedAt.IsZero() {
		return nil
	}

	tracked.Status = taskStatusOf(report.Status)
	tracked.FinishedAt = time.Now().UTC()
	if report.SessionID != "" {
		tracked.SessionID = report.SessionID
	}
	if result := report.Result; result != nil {
		tracked.SessionFailed = result.IsError
		tracked.SessionResult = result.Text
		tracked.SessionTurns = result.NumTurns
		tracked.SessionDuration = result.Duration
		tracked.SessionCostUSD = result.CostUSD
	}

	if err := service.tasks.UpdateTask(projectSlug, tracked); err != nil {
		return fmt.Errorf("the Session of task %s has finished, but the task could not be marked %q: %w", tracked.ID, tracked.Status, err)
	}

	service.logger.Info("Task [%s] %s is %s, its Session is over", tracked.ID, tracked.Title, tracked.Status)
	return nil
}

// taskStatusOf turns the status of a finished Session into the status of the
// task it worked on.
func taskStatusOf(status SessionStatus) task.TaskStatus {
	if status == StatusGotShitDone {
		return task.StatusDone
	}
	return task.StatusFuckedUp
}

func readSessionReport(runDir string, now time.Time) (SessionReport, error) {
	present, err := common.Exists(runDir)
	if err != nil {
		return SessionReport{}, err
	}
	if !present {
		return SessionReport{}, fmt.Errorf("%w at %s", errNoRunDirectory, runDir)
	}

	report := SessionReport{RunDir: runDir}

	if report.SessionID, err = readSessionID(runDir); err != nil {
		return SessionReport{}, err
	}
	if report.LastWrite, err = readLastWrite(runDir); err != nil {
		return SessionReport{}, err
	}

	terminal, err := readResult(runDir)
	if err != nil {
		return SessionReport{}, err
	}
	report.Result = sessionResultOf(terminal)

	finished, err := sessionFinished(runDir)
	if err != nil {
		return SessionReport{}, err
	}
	if !finished {
		report.Status = statusOfRunning(report.LastWrite, now)
		return report, nil
	}

	if report.ExitCode, err = readExitCode(runDir); err != nil {
		return SessionReport{}, err
	}
	report.Status = statusOfFinished(report.ExitCode, report.Result)
	return report, nil
}

// sessionFinished reports whether a Session has stopped.
func sessionFinished(runDir string) (bool, error) {
	finished, err := common.Exists(common.RunExitPath(runDir))
	if err != nil {
		return false, fmt.Errorf("could not tell whether the Session in run directory %s has finished: %w", runDir, err)
	}
	return finished, nil
}

// statusOfRunning judges a Session that has not written its exit file.
func statusOfRunning(lastWrite time.Time, now time.Time) SessionStatus {
	if now.Sub(lastWrite) > sessionStaleAfter {
		return StatusNeedsBabysitting
	}
	return StatusWorking
}

// statusOfFinished judges a Session that has stopped.
func statusOfFinished(exitCode int, result *SessionResult) SessionStatus {
	if exitCode != 0 {
		return StatusFuckedUp
	}
	if result == nil || result.IsError || result.Subtype != streamSubtypeSuccess {
		return StatusFuckedUp
	}
	return StatusGotShitDone
}

// readLastWrite reports when the Session last produced output.
func readLastWrite(runDir string) (time.Time, error) {
	stream, err := os.Stat(common.RunStreamPath(runDir))
	if err == nil {
		return stream.ModTime(), nil
	}
	if !errors.Is(err, fs.ErrNotExist) {
		return time.Time{}, fmt.Errorf("could not check the event stream of run directory %s: %w", runDir, err)
	}

	directory, err := os.Stat(runDir)
	if err != nil {
		return time.Time{}, fmt.Errorf("could not check run directory %s: %w", runDir, err)
	}
	return directory.ModTime(), nil
}

// readExitCode reads what the agent of a finished Session exited with.
func readExitCode(runDir string) (int, error) {
	raw, err := common.ReadFile(common.RunExitPath(runDir))
	if err != nil {
		return 0, err
	}

	written := strings.TrimSpace(raw)
	exitCode, err := strconv.Atoi(written)
	if err != nil {
		return 0, fmt.Errorf("the exit file of run directory %s holds %q, which is not an exit code", runDir, written)
	}
	return exitCode, nil
}

// sessionResultOf keeps the parts of a terminal event drudge reports on. A nil
// event means the agent has not written one yet.
func sessionResultOf(terminal *streamEvent) *SessionResult {
	if terminal == nil {
		return nil
	}
	return &SessionResult{
		IsError:  terminal.IsError,
		Subtype:  terminal.Subtype,
		NumTurns: terminal.NumTurns,
		Duration: time.Duration(terminal.DurationMS) * time.Millisecond,
		CostUSD:  terminal.TotalCostUSD,
		Text:     terminal.Result,
	}
}
