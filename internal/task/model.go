// Package task
package task

import "time"

type (
	TaskID     string // UUID
	TaskStatus string
)

const (
	StatusDraft      = "draft" // Default status
	StatusTodo       = "todo"
	StatusInProgress = "in-progress"
	StatusFuckedUp   = "fucked-up"
	StatusDone       = "done"
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

	CreatedAt time.Time
	UpdatedAt time.Time
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
}
