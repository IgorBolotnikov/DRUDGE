package drudger

import (
	"strings"
	"testing"

	"github.com/IgorBolotnikov/DRUDGE/internal/common"
	"github.com/IgorBolotnikov/DRUDGE/internal/config"
	"github.com/IgorBolotnikov/DRUDGE/internal/task"
)

func TestDrudgerService_RefuseWhileWorking(t *testing.T) {
	cases := []struct {
		name string
		// runDir says what the run directory of the task holds. An empty one
		// stands for a task no agent has been given.
		stream string
		exit   string
		// hasHolder says whether a Drudger is recorded against the task.
		hasHolder bool
		wantErr   bool
	}{
		{
			name:      "an agent still writing",
			stream:    initEvent,
			hasHolder: true,
			wantErr:   true,
		},
		{
			name:      "a Session that has finished",
			stream:    initEvent,
			exit:      "0\n",
			hasHolder: true,
		},
		{
			name:      "a task no agent has been given",
			hasHolder: false,
		},
		{
			name:      "a run whose slot was reclaimed",
			stream:    initEvent,
			hasHolder: false,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			projectDir := setupProjectDir(t)
			taskToEdit := todoTask()

			runDir := common.RunDir(projectDir, string(taskToEdit.ID))
			if testCase.stream != "" {
				writeStream(t, runDir, testCase.stream)
			}
			if testCase.exit != "" {
				writeExit(t, runDir, testCase.exit)
			}

			pool := []*Drudger{idleDrudger(1)}
			if testCase.hasHolder {
				pool[0].TaskID = taskToEdit.ID
			}

			service := newTestServiceWithPool(
				&config.LocalConfig{ProjectSlug: testProjectSlug},
				config.DefaultConfig(),
				&fakeCommandRunner{projectDir: projectDir},
				pool,
				taskToEdit,
			)

			err := service.RefuseWhileWorking(testProjectSlug, taskToEdit)

			if !testCase.wantErr {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}

			if err == nil {
				t.Fatal("expected a task with a working agent to be refused")
			}
			for _, want := range []string{testSandbox, string(taskToEdit.ID), nukeCommand} {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("expected the error to name %q, got %q", want, err)
				}
			}
		})
	}
}

// A run refuses a draft, and an edit is what moves a draft to todo.
func TestDrudgerService_RunTask_TakesATaskEditedIntoTodo(t *testing.T) {
	projectDir := setupProjectDir(t)
	draft := todoTask()
	draft.Status = task.StatusDraft

	commands := &fakeCommandRunner{projectDir: projectDir, outputs: []string{sandboxListingWith(testSandbox)}}
	service := newTestServiceWith(&config.LocalConfig{ProjectSlug: testProjectSlug}, config.DefaultConfig(), commands, draft)
	tasks := task.NewTaskService(service.taskRepo, common.NewLogger(""))

	var err error
	captureOutput(func() { err = service.RunTask(testProjectSlug, draft.ID, false) })
	if err == nil {
		t.Fatal("expected a draft task to be refused")
	}

	captureOutput(func() {
		_, err = tasks.EditTask(testProjectSlug, draft.ID, statusChange(task.StatusTodo), service.DrudgerService)
	})
	if err != nil {
		t.Fatalf("unexpected error editing the draft: %v", err)
	}

	captureOutput(func() { err = service.RunTask(testProjectSlug, draft.ID, false) })
	if err != nil {
		t.Fatalf("unexpected error running the edited task: %v", err)
	}
	if draft.Status != task.StatusInProgress {
		t.Errorf("expected the edited task to be handed to an agent, got %q", draft.Status)
	}
}

func statusChange(status task.TaskStatus) task.EditTaskDto {
	return task.EditTaskDto{Status: &status}
}
