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
	// StatusNeverGotGoing means the vendor turned the agent away, so the work
	// on the task never reallystarted.
	StatusNeverGotGoing SessionStatus = "never got going"
)

// Fragments of a vendor error code that say which kind of refusal it was.
const (
	authErrorFragment        = "auth"
	rateLimitErrorFragment   = "rate_limit"
	vendorOverloadedFragment = "overloaded"
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

	// VendorErrorClass is which kind of refusal the vendor made. It is empty
	// when the vendor refused nothing, so it is also the answer to whether the
	// run was refused at all.
	VendorErrorClass task.VendorErrorClass
}

// Finished reports whether the Session has stopped, whatever it left behind.
func (report SessionReport) Finished() bool {
	switch report.Status {
	case StatusFuckedUp, StatusGotShitDone, StatusNeverGotGoing:
		return true
	default:
		return false
	}
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

// recordOutcome writes what a finished Session left behind onto its task.
func (service *DrudgerService) recordOutcome(projectSlug string, tracked *task.Task, report SessionReport) error {
	if !report.Finished() || !tracked.FinishedAt.IsZero() {
		return nil
	}

	if report.Status == StatusNeverGotGoing {
		return service.rollBackRefusedRun(projectSlug, tracked, report)
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

// rollBackRefusedRun puts a task the vendor never let start back where it came
// from. The Session outcome fields stay empty, since there was no work to
// describe. The report always carries a result here, because the terminal
// event is what named the refusal.
//
// The finish time stays zero, so every later check records the same refusal
// again. The write is identical every time, and a task that is still blocked
// should keep saying so.
func (service *DrudgerService) rollBackRefusedRun(projectSlug string, tracked *task.Task, report SessionReport) error {
	tracked.Status = task.StatusTodo
	tracked.VendorErrorClass = report.Result.VendorErrorClass
	tracked.VendorError = report.Result.Text

	if err := service.tasks.UpdateTask(projectSlug, tracked); err != nil {
		return fmt.Errorf("the vendor refused the run of task %s, but the task could not be put back to %q: %w", tracked.ID, task.StatusTodo, err)
	}

	service.logger.Info("The vendor refused the agent on task [%s] %s (%s): %s", tracked.ID, tracked.Title, tracked.VendorErrorClass, tracked.VendorError)
	service.logger.Info("Nothing ran, so the task is back in %q.", task.StatusTodo)
	service.logger.Info("%s", vendorErrorAdvice(tracked.VendorErrorClass))
	return nil
}

// vendorErrorAdvice tells the user what to do about a refusal.
func vendorErrorAdvice(class task.VendorErrorClass) string {
	switch class {
	case task.VendorErrorAuth:
		return "Log in again, then re-seed the credentials inside sbx. sbx keeps its own copy of the token, and a host login does not refresh it."
	case task.VendorErrorRateLimit:
		return "Run the task again once the vendor lets you through."
	case task.VendorErrorOutage:
		return "Run the task again once the vendor is serving requests."
	default:
		return "Read the error above, fix what it names, then run the task again."
	}
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

	outcome, err := readOutcome(runDir)
	if err != nil {
		return SessionReport{}, err
	}
	report.Result = sessionResultOf(outcome)

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

// statusOfFinished judges a Session that has stopped. A vendor refusal is read
// first, because a refused run exits non-zero and would otherwise read as the
// task failing.
func statusOfFinished(exitCode int, result *SessionResult) SessionStatus {
	if result != nil && result.VendorErrorClass != "" {
		return StatusNeverGotGoing
	}
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
// result means the agent has not written a terminal event yet.
func sessionResultOf(outcome streamOutcome) *SessionResult {
	terminal := outcome.terminal
	if terminal == nil {
		return nil
	}

	result := &SessionResult{
		IsError:  terminal.IsError,
		Subtype:  terminal.Subtype,
		NumTurns: terminal.NumTurns,
		Duration: time.Duration(terminal.DurationMS) * time.Millisecond,
		CostUSD:  terminal.TotalCostUSD,
		Text:     terminal.Result,
	}
	if terminal.vendorRefused() {
		result.VendorErrorClass = vendorErrorClassOf(outcome.errorCode)
	}
	return result
}

// vendorErrorClassOf reads which kind of refusal a vendor error code was. A
// code drudge does not recognise is still a refusal, so it gets its own class
// instead of being folded into one of the known kinds.
func vendorErrorClassOf(errorCode string) task.VendorErrorClass {
	switch {
	case strings.Contains(errorCode, authErrorFragment):
		return task.VendorErrorAuth
	case strings.Contains(errorCode, rateLimitErrorFragment):
		return task.VendorErrorRateLimit
	case strings.Contains(errorCode, vendorOverloadedFragment):
		return task.VendorErrorOutage
	default:
		return task.VendorErrorUnknown
	}
}
