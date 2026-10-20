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
	Repositories []string // Paths of the repositories this task changes, empty for every repository of the project

	TicketID     string // ID of the ticket this task is related to, if any
	ParentTaskID TaskID // ID of the parent task, if any
	ProjectSlug  string // Slug of the project this task belongs to

	BlockedBy    []TaskID // Full ids of the tasks this task waits for
	PullRequests []string // URLs of the pull requests opened for this task

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
	// the lock. isStored says whether the task went through change and was
	// written back.
	TryUpdateTask(projectSlug string, id TaskID, change func(taskToUpdate *Task) error) (isStored bool, err error)
	// DeleteTask removes the task carrying exactly this id. accept runs
	// against the stored task under an exclusive lock on that task, and a task
	// accept refuses stays where it is. isRemoved is false when someone else
	// holds the lock.
	DeleteTask(projectSlug string, id TaskID, accept func(taskToRemove *Task) error) (isRemoved bool, err error)
	// ReadTaskFiles reads every task file of a project as it is stored, each
	// under the lock of its task, and parses it the way a listing does. A file
	// that does not parse fails the whole read with an error naming the file.
	ReadTaskFiles(projectSlug string) ([]TaskFile, error)
	// WriteTaskArchive writes files and manifest into a new archive at path.
	// It refuses a path that already exists with an error matching
	// fs.ErrExist.
	WriteTaskArchive(path string, manifest ArchiveManifest, files []TaskFile) error
	// ReadTaskArchive reads the manifest and every task file of the archive
	// at path, and parses each task file the way a listing does. An archive
	// that is not a zip, has no manifest or holds a manifest or a task file
	// that does not parse fails with BrokenArchiveError.
	ReadTaskArchive(path string) (ArchiveManifest, []TaskFile, error)
	// WriteTask stores a whole task under its own id, as it is given, and
	// changes none of its timestamps. decide runs under an exclusive lock on
	// that task against the stored copy, which is nil when the project has no
	// task with that id. A decide returning ErrTaskUnchanged writes nothing.
	// isWritten says whether the task was written.
	WriteTask(projectSlug string, taskToWrite *Task, decide func(stored *Task) error) (isWritten bool, err error)
}

// ErrTaskUnchanged tells an update that the task needs no write. A change
// callback returns it after reading the stored task and finding nothing to
// record.
var ErrTaskUnchanged = errors.New("task is unchanged")
