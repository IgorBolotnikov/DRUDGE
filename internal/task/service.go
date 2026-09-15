package task

import (
	"fmt"
	"time"

	"drudge/internal/common"
)

// ShortIDLength is how many leading characters of a task id the interfaces
// print. Short enough to be readable and long enough to avoid collisions.
const ShortIDLength = 8

// ShortID cuts a task id down to what the interfaces print. An id already
// that short is left alone.
func ShortID(id TaskID) string {
	text := string(id)
	if len(text) > ShortIDLength {
		return text[:ShortIDLength]
	}
	return text
}

type TaskService struct {
	repo TaskRepository
	log  *common.Logger
}

func NewTaskService(repo TaskRepository, log *common.Logger) *TaskService {
	return &TaskService{repo: repo, log: log}
}

func (service *TaskService) CreateTask(dto CreateTaskDto) (*Task, error) {
	if dto.Title == "" {
		return nil, fmt.Errorf("task title is required")
	}

	if dto.ProjectSlug == "" {
		return nil, fmt.Errorf("task project slug is required")
	}

	if dto.Status == "" {
		dto.Status = StatusDraft
	}

	if dto.CreatedAt.IsZero() {
		dto.CreatedAt = time.Now()
	}

	task, err := service.repo.CreateTask(dto)
	if err != nil {
		return nil, fmt.Errorf("could not create task: %w", err)
	}

	service.log.Info("Created task [%s] %s", task.ID, task.Title)
	return task, nil
}

func (service *TaskService) ListTasks(projectSlug string) ([]*Task, error) {
	return service.repo.ListTasks(projectSlug)
}

// GetTask finds one task by its full id, or by any prefix of an id that names
// a single task. Listings print shortened ids, so a prefix is what a user has
// in front of them.
func (service *TaskService) GetTask(projectSlug string, id TaskID) (*Task, error) {
	if id == "" {
		return nil, ErrNoTaskID
	}
	return service.repo.FindTask(projectSlug, string(id))
}

// UpdateTask hands the stored task to change under an exclusive lock on it and
// writes back what change leaves behind. It waits for a lock another command
// holds. A change returning ErrTaskUnchanged writes nothing.
func (service *TaskService) UpdateTask(projectSlug string, id TaskID, change func(taskToUpdate *Task) error) error {
	if id == "" {
		return ErrNoTaskID
	}
	return service.repo.UpdateTask(projectSlug, id, change)
}

// TryUpdateTask updates a task the way UpdateTask does and gives up when
// another command holds the lock on it. stored says whether the task was
// written back.
func (service *TaskService) TryUpdateTask(projectSlug string, id TaskID, change func(taskToUpdate *Task) error) (stored bool, err error) {
	if id == "" {
		return false, ErrNoTaskID
	}
	return service.repo.TryUpdateTask(projectSlug, id, change)
}
