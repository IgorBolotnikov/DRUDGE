package persistence

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"drudge/internal/common"
	"drudge/internal/task"
)

const (
	TasksDirName = "tasks"
)

type FileTaskRepository struct {
	Project string
}

func NewFileTaskRepository(project string) *FileTaskRepository {
	return &FileTaskRepository{Project: project}
}

func generateTaskID() (task.TaskID, error) {
	id, err := common.GenerateUUID()
	if err != nil {
		return "", fmt.Errorf("could not generate task id: %w", err)
	}

	return task.TaskID(id), nil
}

func (r *FileTaskRepository) taskDir() string {
	home, err := common.HomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(common.ProjectsDir(home), r.Project, TasksDirName)
}

func (r *FileTaskRepository) CreateTask(dto task.CreateTaskDto) (*task.Task, error) {
	id, err := generateTaskID()
	if err != nil {
		return nil, fmt.Errorf("could not generate task id: %w", err)
	}

	tasksDir := r.taskDir()
	if err := common.EnsureDir(tasksDir); err != nil {
		return nil, fmt.Errorf("could not create tasks directory: %w", err)
	}

	filename := fmt.Sprintf("%s %s.md", id, dto.Title)
	taskFilePath := filepath.Join(tasksDir, filename)

	metadata := map[string]string{
		"id":           string(id),
		"title":        dto.Title,
		"status":       string(dto.Status),
		"project_slug": r.Project,
		"created_at":   dto.CreatedAt.Format(time.RFC3339),
	}
	if dto.TicketID != "" {
		metadata["ticket_id"] = dto.TicketID
	}

	if err := common.WriteFileWithFrontMatter(taskFilePath, metadata, dto.Description); err != nil {
		return nil, fmt.Errorf("could not write task file: %w", err)
	}

	return &task.Task{
		ID:          id,
		Title:       dto.Title,
		Description: dto.Description,
		Status:      dto.Status,
		TicketID:    dto.TicketID,
		ProjectSlug: r.Project,
		CreatedAt:   dto.CreatedAt,
	}, nil
}

func (r *FileTaskRepository) parseTaskFromFile(path string) (*task.Task, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("could not read file %s: %w", path, err)
	}

	metadata, content := common.ParseFrontMatter(string(data))

	t := &task.Task{
		Description: content,
	}

	if id, ok := metadata["id"]; ok {
		t.ID = task.TaskID(id)
	}
	if title, ok := metadata["title"]; ok {
		t.Title = title
	}
	if status, ok := metadata["status"]; ok {
		t.Status = task.TaskStatus(status)
	}
	if ticketID, ok := metadata["ticket_id"]; ok {
		t.TicketID = ticketID
	}
	if projectSlug, ok := metadata["project_slug"]; ok {
		t.ProjectSlug = projectSlug
	}
	if createdAt, ok := metadata["created_at"]; ok {
		t.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	}
	if updatedAt, ok := metadata["updated_at"]; ok {
		t.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
	}

	return t, nil
}

func (r *FileTaskRepository) ListTasks(projectSlug string) ([]*task.Task, error) {
	tasksDir := r.taskDir()

	exists, err := common.Exists(tasksDir)
	if err != nil {
		return nil, fmt.Errorf("could not check tasks directory: %w", err)
	}
	if !exists {
		return nil, nil
	}

	entries, err := os.ReadDir(tasksDir)
	if err != nil {
		return nil, fmt.Errorf("could not list tasks directory: %w", err)
	}

	var tasks []*task.Task
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if !strings.HasSuffix(e.Name(), ".md") {
			continue
		}

		t, err := r.parseTaskFromFile(filepath.Join(tasksDir, e.Name()))
		if err != nil {
			continue
		}

		tasks = append(tasks, t)
	}

	return tasks, nil
}

func (r *FileTaskRepository) GetTask(projectSlug string, id task.TaskID) (*task.Task, error) {
	tasksDir := r.taskDir()

	entries, err := os.ReadDir(tasksDir)
	if err != nil {
		return nil, fmt.Errorf("could not list tasks directory: %w", err)
	}

	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if !strings.HasSuffix(e.Name(), ".md") {
			continue
		}

		t, err := r.parseTaskFromFile(filepath.Join(tasksDir, e.Name()))
		if err != nil {
			continue
		}

		if t.ID == id {
			return t, nil
		}
	}

	return nil, fmt.Errorf("task %q not found", id)
}
