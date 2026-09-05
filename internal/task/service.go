package task

import (
	"fmt"
	"time"

	"drudge/internal/common"
)

// ShortIDLength is how many leading characters of a task id the interfaces
// print. Short enough to be readable and long enough to avoid collisions.
const ShortIDLength = 8

type TaskService struct {
	repo TaskRepository
	log  *common.Logger
}

func NewTaskService(repo TaskRepository, log *common.Logger) *TaskService {
	return &TaskService{repo: repo, log: log}
}

func (t *TaskService) CreateTask(dto CreateTaskDto) (*Task, error) {
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

	task, err := t.repo.CreateTask(dto)
	if err != nil {
		return nil, fmt.Errorf("could not create task: %w", err)
	}

	t.log.Info("Created task [%s] %s", task.ID, task.Title)
	return task, nil
}

func (t *TaskService) ListTasks(projectSlug string) ([]*Task, error) {
	return t.repo.ListTasks(projectSlug)
}

// GetTask finds one task by its full id, or by any prefix of an id that names
// a single task. Listings print shortened ids, so a prefix is what a user has
// in front of them.
func (t *TaskService) GetTask(projectSlug string, id TaskID) (*Task, error) {
	if id == "" {
		return nil, ErrNoTaskID
	}
	return t.repo.FindTask(projectSlug, string(id))
}

func (t *TaskService) UpdateTask(projectSlug string, taskToUpdate *Task) error {
	if taskToUpdate.ID == "" {
		return fmt.Errorf("task id is required to update a task")
	}

	if err := t.repo.UpdateTask(projectSlug, taskToUpdate); err != nil {
		return fmt.Errorf("could not update task %s: %w", taskToUpdate.ID, err)
	}

	return nil
}
