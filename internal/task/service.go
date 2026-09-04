package task

import (
	"fmt"
	"time"

	"drudge/internal/common"
)

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

func (t *TaskService) GetTask(projectSlug string, id TaskID) (*Task, error) {
	return t.repo.GetTask(projectSlug, id)
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
