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

	CreatedAt time.Time
	UpdatedAt time.Time
}
