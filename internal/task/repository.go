package task

import (
	"errors"
	"time"
)

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
	// GetTask reads the task carrying exactly this id. It is the lookup for
	// callers that already hold a task id.
	GetTask(projectSlug string, id TaskID) (*Task, error)
	// FindTask looks up the task named by a full id or by a prefix of one.
	// An implementation searches for the match instead of handing back every
	// task, and reports an ambiguous prefix with AmbiguousIDError.
	FindTask(projectSlug string, fullOrPartialID string) (*Task, error)
	// UpdateTask hands the stored task to change under an exclusive lock on
	// that task and writes back what change leaves behind. It waits for a lock
	// someone else holds. A change returning ErrTaskUnchanged writes nothing.
	UpdateTask(projectSlug string, id TaskID, change func(taskToUpdate *Task) error) error
	// TryUpdateTask works like UpdateTask, but gives up when someone else holds
	// the lock. stored says whether the task went through change and was
	// written back.
	TryUpdateTask(projectSlug string, id TaskID, change func(taskToUpdate *Task) error) (stored bool, err error)
}

// ErrTaskUnchanged tells an update that the task needs no write. A change
// callback returns it after reading the stored task and finding nothing to
// record.
var ErrTaskUnchanged = errors.New("task is unchanged")
