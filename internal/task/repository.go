package task

import "time"

type CreateTaskDto struct {
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
}

type TaskRepository interface {
	CreateTask(dto CreateTaskDto) (*Task, error)
	ListTasks(projectSlug string) ([]*Task, error)
	// FindTask looks up the task named by a full id or by a prefix of one.
	// An implementation searches for the match instead of handing back every
	// task, and reports an ambiguous prefix with AmbiguousIDError.
	FindTask(projectSlug string, fullOrPartialID string) (*Task, error)
	UpdateTask(projectSlug string, taskToUpdate *Task) error
}
