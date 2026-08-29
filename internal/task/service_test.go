package task

import (
	"errors"
	"testing"
	"time"

	"drudge/internal/common"
)

type mockRepo struct {
	createTaskFn func(CreateTaskDto) (*Task, error)
	listTasksFn  func(string) ([]*Task, error)
	getTaskFn    func(string, TaskID) (*Task, error)
}

func (m *mockRepo) CreateTask(dto CreateTaskDto) (*Task, error) {
	if m.createTaskFn != nil {
		return m.createTaskFn(dto)
	}
	return nil, nil
}

func (m *mockRepo) ListTasks(projectSlug string) ([]*Task, error) {
	if m.listTasksFn != nil {
		return m.listTasksFn(projectSlug)
	}
	return nil, nil
}

func (m *mockRepo) GetTask(projectSlug string, id TaskID) (*Task, error) {
	if m.getTaskFn != nil {
		return m.getTaskFn(projectSlug, id)
	}
	return nil, nil
}

func TestTaskService_CreateTask_MissingTitle(t *testing.T) {
	repo := &mockRepo{}
	svc := NewTaskService(repo, common.NewLogger(""))

	_, err := svc.CreateTask(CreateTaskDto{
		ProjectSlug: "test",
		Status:      StatusTodo,
	})

	if err == nil {
		t.Fatal("expected error for missing title")
	}
}

func TestTaskService_CreateTask_MissingProjectSlug(t *testing.T) {
	repo := &mockRepo{}
	svc := NewTaskService(repo, common.NewLogger(""))

	_, err := svc.CreateTask(CreateTaskDto{
		Title:  "Fix bug",
		Status: StatusTodo,
	})

	if err == nil {
		t.Fatal("expected error for missing project slug")
	}
}

func TestTaskService_CreateTask_DefaultsStatusToDraft(t *testing.T) {
	repo := &mockRepo{
		createTaskFn: func(dto CreateTaskDto) (*Task, error) {
			if dto.Status != StatusDraft {
				t.Errorf("expected status %q, got %q", StatusDraft, dto.Status)
			}
			return &Task{ID: "abc123", Title: dto.Title}, nil
		},
	}
	svc := NewTaskService(repo, common.NewLogger(""))

	_, err := svc.CreateTask(CreateTaskDto{
		Title:       "Fix bug",
		ProjectSlug: "test",
		Status:      "",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestTaskService_CreateTask_DefaultsCreatedAt(t *testing.T) {
	repo := &mockRepo{
		createTaskFn: func(dto CreateTaskDto) (*Task, error) {
			if dto.CreatedAt.IsZero() {
				t.Error("expected CreatedAt to be set")
			}
			return &Task{ID: "abc123", Title: dto.Title}, nil
		},
	}
	svc := NewTaskService(repo, common.NewLogger(""))

	_, err := svc.CreateTask(CreateTaskDto{
		Title:       "Fix bug",
		ProjectSlug: "test",
		Status:      StatusTodo,
		CreatedAt:   time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestTaskService_CreateTask_PreservesExplicitCreatedAt(t *testing.T) {
	fixedTime := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)
	repo := &mockRepo{
		createTaskFn: func(dto CreateTaskDto) (*Task, error) {
			if !dto.CreatedAt.Equal(fixedTime) {
				t.Errorf("expected CreatedAt %v, got %v", fixedTime, dto.CreatedAt)
			}
			return &Task{ID: "abc123", Title: dto.Title}, nil
		},
	}
	svc := NewTaskService(repo, common.NewLogger(""))

	_, err := svc.CreateTask(CreateTaskDto{
		Title:       "Fix bug",
		ProjectSlug: "test",
		Status:      StatusTodo,
		CreatedAt:   fixedTime,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestTaskService_CreateTask_ForwardsToRepo(t *testing.T) {
	expected := &Task{ID: "abc123", Title: "Fix login", Description: "SSO broken", Status: StatusTodo}
	repo := &mockRepo{
		createTaskFn: func(dto CreateTaskDto) (*Task, error) {
			return expected, nil
		},
	}
	svc := NewTaskService(repo, common.NewLogger(""))

	result, err := svc.CreateTask(CreateTaskDto{
		Title:       "Fix login",
		Description: "SSO broken",
		Status:      StatusTodo,
		ProjectSlug: "test",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ID != expected.ID || result.Title != expected.Title {
		t.Errorf("expected %+v, got %+v", expected, result)
	}
}

func TestTaskService_CreateTask_WrapsRepoError(t *testing.T) {
	repoErr := errors.New("db error")
	repo := &mockRepo{
		createTaskFn: func(dto CreateTaskDto) (*Task, error) {
			return nil, repoErr
		},
	}
	svc := NewTaskService(repo, common.NewLogger(""))

	_, err := svc.CreateTask(CreateTaskDto{
		Title:       "Fix bug",
		ProjectSlug: "test",
		Status:      StatusTodo,
	})

	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, repoErr) {
		t.Errorf("expected wrapped repo error, got %v", err)
	}
}

func TestTaskService_ListTasks_ForwardsToRepo(t *testing.T) {
	expected := []*Task{{ID: "1", Title: "Task One"}, {ID: "2", Title: "Task Two"}}
	repo := &mockRepo{
		listTasksFn: func(string) ([]*Task, error) {
			return expected, nil
		},
	}
	svc := NewTaskService(repo, common.NewLogger(""))

	result, err := svc.ListTasks("test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 2 {
		t.Fatalf("expected 2 tasks, got %d", len(result))
	}
}

func TestTaskService_GetTask_ForwardsToRepo(t *testing.T) {
	expected := &Task{ID: "abc123", Title: "Get Me"}
	repo := &mockRepo{
		getTaskFn: func(string, TaskID) (*Task, error) {
			return expected, nil
		},
	}
	svc := NewTaskService(repo, common.NewLogger(""))

	result, err := svc.GetTask("test", "abc123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ID != expected.ID {
		t.Errorf("expected ID %s, got %s", expected.ID, result.ID)
	}
}
