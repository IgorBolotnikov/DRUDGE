package drudger

import (
	"time"

	"github.com/IgorBolotnikov/DRUDGE/internal/task"
)

// DrudgerRepository stores the Drudgers a project has. An entry appears when a
// Drudger is first claimed and leaves only by deliberate destructionl. The
// store usually does not grow beyond the bounds of the pool.
type DrudgerRepository interface {
	// ListDrudgers reads the Drudgers of a project.
	ListDrudgers(projectSlug string) ([]*Drudger, error)
	// UpdateDrudgers hands the project's Drudgers to change to the callback and
	// stores what it returns. The whole call holds an exclusive lock. An error
	// from change stores nothing and is returned back to the caller.
	UpdateDrudgers(projectSlug string, change func(drudgers []*Drudger) ([]*Drudger, error)) error
	// TryUpdateDrudgers works like UpdateDrudgers, but gives up when someone
	// else holds the lock. isStored says whether the Drudgers went through change
	// and were written back.
	TryUpdateDrudgers(projectSlug string, change func(drudgers []*Drudger) ([]*Drudger, error)) (isStored bool, err error)
}

// RunRepository stores the latest run of each task. A run holds the prompt the
// agent is given, and the event stream, the stderr log and the exit code the
// agent leaves behind. The agent writes its files from inside its sandbox, so
// the repository only reads them. A run also holds the pull request
// descriptions close-out has not opened a pull request from yet, keyed by
// repository name.
type RunRepository interface {
	// PrepareRun deletes whatever an earlier run of a task left and stores the
	// prompt of a fresh run.
	PrepareRun(taskID task.TaskID, prompt string) error
	// HasRun tells whether a task has a run.
	HasRun(taskID task.TaskID) (bool, error)
	// ReadStream returns the event stream of a run as far as the agent has
	// written it. isPresent is false while the agent has not created it.
	ReadStream(taskID task.TaskID) (content []byte, isPresent bool, err error)
	// LastWrite returns when the event stream of a run was last written, or
	// when the run was prepared if the agent has not created the stream. It
	// refuses a task with no run.
	LastWrite(taskID task.TaskID) (time.Time, error)
	// ReadExit returns what the launcher wrote to the exit file of a run.
	// hasExited is false while the agent has not exited.
	ReadExit(taskID task.TaskID) (written string, hasExited bool, err error)
	// ReadStderr returns the stderr log of a run. isPresent is false while the
	// agent has not created it.
	ReadStderr(taskID task.TaskID) (content []byte, isPresent bool, err error)
	// WritePullRequest stores the pull request description of one repository.
	// It refuses a task with no run.
	WritePullRequest(taskID task.TaskID, repository string, description string) error
	// ReadPullRequest returns the pull request description of one repository.
	// isPresent is false when the run holds none.
	ReadPullRequest(taskID task.TaskID, repository string) (description string, isPresent bool, err error)
	// ListPullRequests names the repositories whose pull request description
	// the run holds, sorted by name. A task with no run holds none.
	ListPullRequests(taskID task.TaskID) ([]string, error)
	// RemovePullRequest deletes the pull request description of one
	// repository. One that is not there is not an error.
	RemovePullRequest(taskID task.TaskID, repository string) error
	// RemoveRun deletes the run of a task and reports whether it had one.
	RemoveRun(taskID task.TaskID) (isRemoved bool, err error)
}
