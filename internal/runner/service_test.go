package runner

import (
	"fmt"
	"io"
	"os"
	"strings"
	"testing"

	"drudge/internal/common"
	"drudge/internal/config"
	"drudge/internal/task"
)

const testProjectSlug = "test-project"

type fakeTaskRepo struct {
	tasks []*task.Task
}

func (repo *fakeTaskRepo) CreateTask(dto task.CreateTaskDto) (*task.Task, error) {
	return nil, fmt.Errorf("CreateTask should not be called")
}

func (repo *fakeTaskRepo) ListTasks(projectSlug string) ([]*task.Task, error) {
	return repo.tasks, nil
}

func (repo *fakeTaskRepo) GetTask(projectSlug string, id task.TaskID) (*task.Task, error) {
	for _, candidate := range repo.tasks {
		if candidate.ID == id {
			return candidate, nil
		}
	}
	return nil, fmt.Errorf("task %q not found", id)
}

func newTestService(tasks ...*task.Task) *RunnerService {
	logger := common.NewLogger("")
	return New(logger, config.DefaultConfig(), task.NewTaskService(&fakeTaskRepo{tasks: tasks}, logger))
}

func captureOutput(f func()) string {
	orig := os.Stdout
	reader, writer, _ := os.Pipe()
	os.Stdout = writer
	f()
	writer.Close()
	os.Stdout = orig
	out, _ := io.ReadAll(reader)
	return string(out)
}

func TestRunnerService_RunTask_DryRunStatusGuard(t *testing.T) {
	cases := []struct {
		name    string
		status  task.TaskStatus
		wantErr bool
	}{
		{name: "runs a todo task", status: task.StatusTodo},
		{name: "refuses a draft task", status: task.StatusDraft, wantErr: true},
		{name: "refuses an in-progress task", status: task.StatusInProgress, wantErr: true},
		{name: "refuses a fucked-up task", status: task.StatusFuckedUp, wantErr: true},
		{name: "refuses a done task", status: task.StatusDone, wantErr: true},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			service := newTestService(&task.Task{
				ID:          "task-1",
				Title:       "Fix login",
				Description: "SSO is broken",
				Status:      testCase.status,
				ProjectSlug: testProjectSlug,
			})

			var err error
			captureOutput(func() { err = service.RunTask(testProjectSlug, "task-1", true) })

			if testCase.wantErr {
				if err == nil {
					t.Fatalf("expected an error for status %q", testCase.status)
				}
				if !strings.Contains(err.Error(), string(testCase.status)) {
					t.Errorf("expected error to name status %q, got %q", testCase.status, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestRunnerService_RunTask_DryRunPrintsPrompt(t *testing.T) {
	taskToRun := &task.Task{
		ID:          "task-1",
		Title:       "Fix login",
		Description: "SSO is broken",
		TicketID:    "PROJ-123",
		Status:      task.StatusTodo,
		ProjectSlug: testProjectSlug,
	}
	service := newTestService(taskToRun)

	var err error
	out := captureOutput(func() { err = service.RunTask(testProjectSlug, taskToRun.ID, true) })
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, want := range []string{taskToRun.Title, taskToRun.Description, taskToRun.TicketID} {
		if !strings.Contains(out, want) {
			t.Errorf("expected dry run output to contain %q, got %q", want, out)
		}
	}
}

func TestRunnerService_RunTask_UnknownTask(t *testing.T) {
	service := newTestService()

	err := service.RunTask(testProjectSlug, "nope", true)
	if err == nil {
		t.Fatal("expected an error for an unknown task ID")
	}
}

func TestRunnerService_RunTask_WithoutDryRunIsNotImplemented(t *testing.T) {
	service := newTestService(&task.Task{
		ID:          "task-1",
		Title:       "Fix login",
		Description: "SSO is broken",
		Status:      task.StatusTodo,
		ProjectSlug: testProjectSlug,
	})

	err := service.RunTask(testProjectSlug, "task-1", false)
	if err == nil {
		t.Fatal("expected an error, spawning an agent is not implemented yet")
	}
}
