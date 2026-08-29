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
	UpdatedAt time.Time
}

type TaskRepository interface {
	CreateTask(dto CreateTaskDto) (*Task, error)
	ListTasks(projectSlug string) ([]*Task, error)
	GetTask(projectSlug string, id TaskID) (*Task, error)
}
