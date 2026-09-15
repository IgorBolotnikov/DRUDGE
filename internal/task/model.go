// Package task
package task

import (
	"slices"
	"strings"
	"time"
)

type (
	TaskID     string // UUID
	TaskStatus string
	// VendorErrorClass is which kind of refusal a vendor made when it turned a run away
	VendorErrorClass string
)

const (
	StatusDraft      TaskStatus = "draft" // Default status
	StatusTodo       TaskStatus = "todo"
	StatusInProgress TaskStatus = "in-progress"
	StatusFuckedUp   TaskStatus = "fucked-up"
	StatusDone       TaskStatus = "done"
)

// Kinds of vendor refusal. An empty class means the vendor refused nothing.
const (
	VendorErrorAuth      VendorErrorClass = "auth"
	VendorErrorRateLimit VendorErrorClass = "rate limit"
	VendorErrorOutage    VendorErrorClass = "outage"
	VendorErrorUnknown   VendorErrorClass = "unknown" // A refusal drudge cannot tell apart
)

type Task struct {
	ID           TaskID
	Title        string
	Description  string // Markdown
	Status       TaskStatus
	StartedAt    time.Time
	FinishedAt   time.Time
	Repositories []string // List of repositories this task is related to

	TicketID     string // ID of the ticket this task is related to, if any
	ParentTaskID TaskID // ID of the parent task, if any
	ProjectSlug  string // Slug of the project this task belongs to

	SessionID string // Resumable agent session of the run, empty until the agent reports it

	// What the agent reported when its run ended. All of it stays zero until
	// drudge observes a finished run.
	SessionFailed   bool          // Whether the agent flagged its own run as an error
	SessionResult   string        // The last thing the agent said
	SessionTurns    int           // How many turns the agent took
	SessionDuration time.Duration // How long the agent worked
	SessionCostUSD  float64       // What the run cost

	// Stashes is the commit of the stash a handover made in each repository
	// of the workspace, keyed by repository name. It holds what the run
	// before this one left uncommitted.
	Stashes map[string]string

	// Why the vendor turned the last run away, when it did. A refused run did
	// no work, so this is kept apart from what the agent reported.
	VendorError      string           // What the vendor said when it refused the run
	VendorErrorClass VendorErrorClass // Which kind of refusal that was

	CreatedAt time.Time
	UpdatedAt time.Time
}

// Statuses are every status a task can carry.
var Statuses = []TaskStatus{StatusDraft, StatusTodo, StatusInProgress, StatusFuckedUp, StatusDone}

// KnownStatus reports whether a status is one drudge understands.
func KnownStatus(status TaskStatus) bool {
	return slices.Contains(Statuses, status)
}

// FormatStatuses joins statuses into a list for a message to the user.
func FormatStatuses(statuses []TaskStatus) string {
	names := make([]string, 0, len(statuses))
	for _, status := range statuses {
		names = append(names, string(status))
	}
	return strings.Join(names, ", ")
}

// StartRun marks a task as handed to an agent, and clears what the previous
// run left behind. A task only carries the outcome of its current run.
func (taskToRun *Task) StartRun(startedAt time.Time, sessionID string) {
	taskToRun.Status = StatusInProgress
	taskToRun.StartedAt = startedAt
	taskToRun.SessionID = sessionID

	taskToRun.FinishedAt = time.Time{}
	taskToRun.SessionFailed = false
	taskToRun.SessionResult = ""
	taskToRun.SessionTurns = 0
	taskToRun.SessionDuration = 0
	taskToRun.SessionCostUSD = 0
	taskToRun.VendorError = ""
	taskToRun.VendorErrorClass = ""
	taskToRun.Stashes = nil
}

// RecordStash stores the commit of the stash a handover made in one
// repository. An empty commit records nothing, which is what a worktree that
// was already clean gets.
func (taskToRun *Task) RecordStash(repository string, commit string) {
	if commit == "" {
		return
	}
	if taskToRun.Stashes == nil {
		taskToRun.Stashes = map[string]string{}
	}
	taskToRun.Stashes[repository] = commit
}

// HasRun reports whether an agent has ever been handed this task. A launch
// stamps the start time and StartRun clears everything the run before it left.
func (taskToRead *Task) HasRun() bool {
	return !taskToRead.StartedAt.IsZero()
}

// RunFinished reports whether drudge has seen the last run end. The fields the
// agent reported carry its outcome only once this is true.
func (taskToRead *Task) RunFinished() bool {
	return !taskToRead.FinishedAt.IsZero()
}

// WasRefused reports whether the vendor turned the last run away. A refused
// run did no work, so it reports no outcome of its own.
func (taskToRead *Task) WasRefused() bool {
	return taskToRead.VendorError != ""
}

// AgentReported reports whether the agent said anything about the last run.
// A run killed with its Drudger is recorded as finished without a report, so
// every field the agent fills stays zero.
func (taskToRead *Task) AgentReported() bool {
	return taskToRead.SessionTurns > 0 || taskToRead.SessionResult != ""
}
