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

// SessionVerdict is how the work of one Session is going. It answers a
// different question from the Task's own status and from the Drudger's health,
// so it is its own type and never mixed with either.
type SessionVerdict string

const (
	// VerdictWorking means the agent is still running and still writing.
	VerdictWorking SessionVerdict = "working"
	// VerdictNeedsBabysitting means the agent is still running but has gone
	// quiet for long enough to be worth a look.
	VerdictNeedsBabysitting SessionVerdict = "needs babysitting"
	// VerdictFuckedUp means the agent stopped without doing the work.
	VerdictFuckedUp SessionVerdict = "fucked up"
	// VerdictGotShitDone means the agent finished the work it was given.
	VerdictGotShitDone SessionVerdict = "got shit done"
)

// sessionStaleAfter is how long a running Session may write nothing before
// drudge says it needs babysitting. A single tool call can take minutes on its
// own, so a tighter threshold would cry wolf on healthy runs. Complete silence
// this long has nothing normal behind it.
const sessionStaleAfter = 5 * time.Minute

// errNoRunDirectory reports a task that has no run directory, so there is no
// Session to say anything about.
var errNoRunDirectory = errors.New("no run directory")

// SessionReport is what the run directory of a task says about the last
// Session that worked on it.
type SessionReport struct {
	Verdict   SessionVerdict
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
	return report.Verdict == VerdictFuckedUp || report.Verdict == VerdictGotShitDone
}

// TaskSession pairs a task with what its run directory says about the last
// Session that worked on it.
type TaskSession struct {
	Task   *task.Task
	Report SessionReport
}

// SessionStatus tells how the last Session of a task is going. The workspace
// is a live bind mount into the sandbox, so the answer comes out of ordinary
// file reads and no sandbox command is involved.
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

	return &TaskSession{Task: tracked, Report: report}, nil
}

// readSessionReport works out how a Session is going from the files its run
// directory holds.
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
		report.Verdict = verdictOfRunning(report.LastWrite, now)
		return report, nil
	}

	if report.ExitCode, err = readExitCode(runDir); err != nil {
		return SessionReport{}, err
	}
	report.Verdict = verdictOfFinished(report.ExitCode, report.Result)
	return report, nil
}

// sessionFinished reports whether a Session has stopped. The launcher writes
// the exit file once the agent is gone, so its presence is the only marker of
// a finished run.
func sessionFinished(runDir string) (bool, error) {
	finished, err := common.Exists(common.RunExitPath(runDir))
	if err != nil {
		return false, fmt.Errorf("could not tell whether the Session in run directory %s has finished: %w", runDir, err)
	}
	return finished, nil
}

// verdictOfRunning judges a Session that has not written its exit file. An
// agent appends to its event stream as it works, so a stream that stopped
// growing is the only sign of trouble drudge can see from outside.
func verdictOfRunning(lastWrite time.Time, now time.Time) SessionVerdict {
	if now.Sub(lastWrite) > sessionStaleAfter {
		return VerdictNeedsBabysitting
	}
	return VerdictWorking
}

// verdictOfFinished judges a Session that has stopped. Only an agent that
// exited cleanly and said so on its terminal event got the work done.
func verdictOfFinished(exitCode int, result *SessionResult) SessionVerdict {
	if exitCode != 0 {
		return VerdictFuckedUp
	}
	if result == nil || result.IsError || result.Subtype != streamSubtypeSuccess {
		return VerdictFuckedUp
	}
	return VerdictGotShitDone
}

// readLastWrite says when the Session last produced output. The event stream
// is the heartbeat, since the agent appends to it as it works. Until the agent
// writes its first event there is no stream, and the run directory stands in,
// because drudge creates it right before the launch.
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
// event means the agent has not written one yet, and a nil result says so.
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
