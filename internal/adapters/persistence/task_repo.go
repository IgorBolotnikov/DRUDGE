package persistence

import (
	"fmt"

	"drudge/internal/common"
	"drudge/internal/task"
)

type FileTaskRepository struct {
	Project string
}

func NewFileTaskRepository(project string) *FileTaskRepository {
	return &FileTaskRepository{Project: project}
}

func (r *FileTaskRepository) CreateTask(dto task.CreateTaskDto) (*task.Task, error) {
	return nil, nil
}

func generateTaskID() (task.TaskID, error) {
	id, err := common.GenerateUUID()
	if err != nil {
		return "", fmt.Errorf("could not generate task id: %w", err)
	}

	return task.TaskID(id), nil
}
