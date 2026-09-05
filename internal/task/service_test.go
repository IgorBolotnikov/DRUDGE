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
	findTaskFn   func(string, string) (*Task, error)
	updateTaskFn func(string, *Task) error
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

func (m *mockRepo) FindTask(projectSlug string, fullOrPartialID string) (*Task, error) {
	if m.findTaskFn != nil {
		return m.findTaskFn(projectSlug, fullOrPartialID)
	}
	return nil, nil
}

func (m *mockRepo) UpdateTask(projectSlug string, taskToUpdate *Task) error {
	if m.updateTaskFn != nil {
		return m.updateTaskFn(projectSlug, taskToUpdate)
	}
	return nil
}

func TestTaskService_UpdateTask(t *testing.T) {
	cases := []struct {
		name       string
		taskToSave *Task
		repoErr    error
		wantErr    bool
		wantCall   bool
	}{
		{
			name:       "saves the task",
			taskToSave: &Task{ID: "task-1", Title: "Fix login", Status: StatusInProgress},
			wantCall:   true,
		},
		{
			name:       "refuses a task without an id",
			taskToSave: &Task{Title: "Fix login"},
			wantErr:    true,
		},
		{
			name:       "surfaces a repository error",
			taskToSave: &Task{ID: "task-1", Title: "Fix login"},
			repoErr:    errors.New("disk is on fire"),
			wantErr:    true,
			wantCall:   true,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			called := false
			repo := &mockRepo{updateTaskFn: func(projectSlug string, taskToUpdate *Task) error {
				called = true
				return testCase.repoErr
			}}
			svc := NewTaskService(repo, common.NewLogger(""))

			err := svc.UpdateTask("test", testCase.taskToSave)

			if testCase.wantErr && err == nil {
				t.Fatal("expected an error")
			}
			if !testCase.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if called != testCase.wantCall {
				t.Errorf("expected the repository to be called: %v, got %v", testCase.wantCall, called)
			}
		})
	}
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

func TestTaskService_GetTask_HandsTheIDToTheRepository(t *testing.T) {
	// Resolving a full id or a prefix is the repository's job. The service
	// passes on whatever the caller typed and returns what comes back.
	cases := []struct {
		name string
		id   TaskID
	}{
		{name: "a full id", id: "006684e3-dbe9-4316-8aba-8a67a8f01f8f"},
		{name: "the prefix a listing prints", id: "006684e3"},
		{name: "a couple of characters", id: "00"},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			found := &Task{ID: "006684e3-dbe9-4316-8aba-8a67a8f01f8f", Title: "Fix login"}
			var asked string
			repo := &mockRepo{findTaskFn: func(_ string, id string) (*Task, error) {
				asked = id
				return found, nil
			}}
			service := NewTaskService(repo, common.NewLogger(""))

			got, err := service.GetTask("demo", testCase.id)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if asked != string(testCase.id) {
				t.Errorf("expected the repository to be asked for %q, got %q", testCase.id, asked)
			}
			if got != found {
				t.Errorf("expected the task the repository found, got %v", got)
			}
		})
	}
}

func TestTaskService_GetTask_RefusesAnEmptyID(t *testing.T) {
	called := false
	repo := &mockRepo{findTaskFn: func(string, string) (*Task, error) {
		called = true
		return nil, nil
	}}
	service := NewTaskService(repo, common.NewLogger(""))

	_, err := service.GetTask("demo", "")
	if !errors.Is(err, ErrNoTaskID) {
		t.Fatalf("expected %v, got %v", ErrNoTaskID, err)
	}
	if called {
		t.Error("expected no lookup for an empty id")
	}
}

func TestTaskService_GetTask_SurfacesTheLookupFailure(t *testing.T) {
	repo := &mockRepo{findTaskFn: func(string, string) (*Task, error) {
		return nil, errors.New("disk on fire")
	}}
	service := NewTaskService(repo, common.NewLogger(""))

	if _, err := service.GetTask("demo", "006684e3"); err == nil {
		t.Fatal("expected the lookup failure to surface")
	}
}
