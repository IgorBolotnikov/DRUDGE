package task

import (
	"errors"
	"strings"
	"testing"
	"time"

	"drudge/internal/common"
)

type mockRepo struct {
	createTaskFn func(CreateTaskDto) (*Task, error)
	listTasksFn  func(string) ([]*Task, error)
	getTaskFn    func(string, TaskID) (*Task, error)
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

func (m *mockRepo) GetTask(projectSlug string, id TaskID) (*Task, error) {
	if m.getTaskFn != nil {
		return m.getTaskFn(projectSlug, id)
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

// Tasks whose ids overlap, so a prefix can be unambiguous, ambiguous, or an
// exact id that is also the prefix of a longer one.
func idResolutionTasks() []*Task {
	return []*Task{
		{ID: "006684e3-dbe9-4316-8aba-8a67a8f01f8f", Title: "Fix login"},
		{ID: "00668f11-1111-4316-8aba-8a67a8f01f8f", Title: "Fix logout"},
		{ID: "abcd1234-2222-4316-8aba-8a67a8f01f8f", Title: "Ship it"},
		{ID: "abcd", Title: "Short id"},
	}
}

func TestTaskService_GetTask_ResolvesFullAndPartialIDs(t *testing.T) {
	cases := []struct {
		name    string
		id      TaskID
		wantID  TaskID
		wantErr string
	}{
		{name: "a full id", id: "006684e3-dbe9-4316-8aba-8a67a8f01f8f", wantID: "006684e3-dbe9-4316-8aba-8a67a8f01f8f"},
		{name: "the prefix a listing prints", id: "006684e3", wantID: "006684e3-dbe9-4316-8aba-8a67a8f01f8f"},
		{name: "a prefix matching nothing", id: "9", wantErr: "not found"},
		{name: "a prefix of a single task", id: "abcd1", wantID: "abcd1234-2222-4316-8aba-8a67a8f01f8f"},
		{name: "an exact id that is also a prefix", id: "abcd", wantID: "abcd"},
		{name: "an uppercase prefix", id: "006684E3", wantID: "006684e3-dbe9-4316-8aba-8a67a8f01f8f"},
		{name: "a prefix matching several tasks", id: "00668", wantErr: "matches 2 tasks"},
		{name: "an unknown id", id: "deadbeef", wantErr: "not found"},
		{name: "an empty id", id: "", wantErr: "task id is required"},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			tasks := idResolutionTasks()
			repo := &mockRepo{listTasksFn: func(string) ([]*Task, error) { return tasks, nil }}
			service := NewTaskService(repo, common.NewLogger(""))

			found, err := service.GetTask("demo", testCase.id)

			if testCase.wantErr != "" {
				if err == nil {
					t.Fatalf("expected an error for id %q", testCase.id)
				}
				if !strings.Contains(err.Error(), testCase.wantErr) {
					t.Errorf("expected error to mention %q, got %q", testCase.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if found.ID != testCase.wantID {
				t.Errorf("expected task %q, got %q", testCase.wantID, found.ID)
			}
		})
	}
}

func TestTaskService_GetTask_AmbiguousIDNamesEveryMatch(t *testing.T) {
	tasks := idResolutionTasks()
	repo := &mockRepo{listTasksFn: func(string) ([]*Task, error) { return tasks, nil }}
	service := NewTaskService(repo, common.NewLogger(""))

	_, err := service.GetTask("demo", "00668")
	if err == nil {
		t.Fatal("expected an error for an ambiguous id")
	}
	// A user picks the right task off this message, so both ids and both
	// titles have to be in it.
	for _, want := range []string{"006684e3-dbe9-4316-8aba-8a67a8f01f8f", "00668f11-1111-4316-8aba-8a67a8f01f8f", "Fix login", "Fix logout"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("expected error to name %q, got %q", want, err)
		}
	}
}

func TestTaskService_GetTask_ReadFailure(t *testing.T) {
	repo := &mockRepo{listTasksFn: func(string) ([]*Task, error) { return nil, errors.New("disk on fire") }}
	service := NewTaskService(repo, common.NewLogger(""))

	if _, err := service.GetTask("demo", "006684e3"); err == nil {
		t.Fatal("expected the read failure to surface")
	}
}
