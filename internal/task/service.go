package task

import (
	"fmt"
	"strings"
	"time"

	"drudge/internal/common"
)

// ShortIDLength is how many leading characters of a task id the interfaces
// print. Whatever a listing shows, a user types back at drudge, so a prefix
// this long has to be enough to name a task.
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
	tasks, err := t.repo.ListTasks(projectSlug)
	if err != nil {
		return nil, fmt.Errorf("could not read the project's tasks to look for task %s: %w", id, err)
	}
	return matchTaskID(tasks, id)
}

// matchTaskID picks the one task an id or an id prefix names. An id that
// matches a task exactly wins, even when it is also the prefix of a longer
// one. Several matches are an error naming all of them, so a user can pick.
func matchTaskID(tasks []*Task, id TaskID) (*Task, error) {
	if id == "" {
		return nil, fmt.Errorf("task id is required")
	}
	wanted := strings.ToLower(string(id))

	var matches []*Task
	for _, candidate := range tasks {
		candidateID := strings.ToLower(string(candidate.ID))
		if candidateID == wanted {
			return candidate, nil
		}
		if strings.HasPrefix(candidateID, wanted) {
			matches = append(matches, candidate)
		}
	}

	switch len(matches) {
	case 0:
		return nil, fmt.Errorf("task %q not found", id)
	case 1:
		return matches[0], nil
	default:
		return nil, fmt.Errorf("task id %q matches %d tasks, add more characters to pick one:\n%s", id, len(matches), formatTaskCandidates(matches))
	}
}

// formatTaskCandidates lists the tasks an ambiguous id prefix matched.
func formatTaskCandidates(matches []*Task) string {
	lines := make([]string, 0, len(matches))
	for _, candidate := range matches {
		lines = append(lines, fmt.Sprintf("  %s  %s", candidate.ID, candidate.Title))
	}
	return strings.Join(lines, "\n")
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
