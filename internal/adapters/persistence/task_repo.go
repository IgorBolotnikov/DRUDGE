package persistence

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"drudge/internal/common"
	"drudge/internal/task"
)

const (
	TasksDirName = "tasks"
)

// Front matter keys of a task file.
const (
	metaKeyID              = "id"
	metaKeyTitle           = "title"
	metaKeyStatus          = "status"
	metaKeyTicketID        = "ticket_id"
	metaKeyProjectSlug     = "project_slug"
	metaKeyRunnerID        = "runner_id"
	metaKeyRunnerSessionID = "runner_session_id"
	metaKeyCreatedAt       = "created_at"
	metaKeyUpdatedAt       = "updated_at"
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

	created := &task.Task{
		ID:          id,
		Title:       dto.Title,
		Description: dto.Description,
		Status:      dto.Status,
		TicketID:    dto.TicketID,
		ProjectSlug: r.Project,
		CreatedAt:   dto.CreatedAt,
	}

	if err := common.WriteFileWithFrontMatter(taskFilePath, taskFrontMatter(created), created.Description); err != nil {
		return nil, fmt.Errorf("could not write task file: %w", err)
	}

	return created, nil
}

// taskFrontMatter serializes a task's fields into front matter keys. Optional
// fields are written only when set, so a task file carries no empty entries.
func taskFrontMatter(taskToWrite *task.Task) map[string]string {
	metadata := map[string]string{
		metaKeyID:          string(taskToWrite.ID),
		metaKeyTitle:       taskToWrite.Title,
		metaKeyStatus:      string(taskToWrite.Status),
		metaKeyProjectSlug: taskToWrite.ProjectSlug,
		metaKeyCreatedAt:   taskToWrite.CreatedAt.Format(time.RFC3339),
	}
	if taskToWrite.TicketID != "" {
		metadata[metaKeyTicketID] = taskToWrite.TicketID
	}
	if taskToWrite.RunnerID != 0 {
		metadata[metaKeyRunnerID] = strconv.Itoa(taskToWrite.RunnerID)
	}
	if taskToWrite.RunnerSessionID != "" {
		metadata[metaKeyRunnerSessionID] = taskToWrite.RunnerSessionID
	}
	if !taskToWrite.UpdatedAt.IsZero() {
		metadata[metaKeyUpdatedAt] = taskToWrite.UpdatedAt.Format(time.RFC3339)
	}
	return metadata
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

	if id, ok := metadata[metaKeyID]; ok {
		t.ID = task.TaskID(id)
	}
	if title, ok := metadata[metaKeyTitle]; ok {
		t.Title = title
	}
	if status, ok := metadata[metaKeyStatus]; ok {
		t.Status = task.TaskStatus(status)
	}
	if ticketID, ok := metadata[metaKeyTicketID]; ok {
		t.TicketID = ticketID
	}
	if projectSlug, ok := metadata[metaKeyProjectSlug]; ok {
		t.ProjectSlug = projectSlug
	}
	if runnerID, ok := metadata[metaKeyRunnerID]; ok {
		parsed, err := strconv.Atoi(runnerID)
		if err != nil {
			return nil, fmt.Errorf("task file %s has %s = %q, it must be a whole number", path, metaKeyRunnerID, runnerID)
		}
		t.RunnerID = parsed
	}
	if runnerSessionID, ok := metadata[metaKeyRunnerSessionID]; ok {
		t.RunnerSessionID = runnerSessionID
	}
	if createdAt, ok := metadata[metaKeyCreatedAt]; ok {
		t.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	}
	if updatedAt, ok := metadata[metaKeyUpdatedAt]; ok {
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
			return nil, err
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
			return nil, err
		}

		if t.ID == id {
			return t, nil
		}
	}

	return nil, fmt.Errorf("task %q not found", id)
}
