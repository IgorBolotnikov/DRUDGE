package tui

import (
	"errors"
	"slices"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/IgorBolotnikov/DRUDGE/internal/task"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/actions"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/tuitest"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/views"
)

var (
	keyNewTask  = tea.KeyPressMsg{Code: 'n', Text: "n"}
	keyEditTask = tea.KeyPressMsg{Code: 'e', Text: "e"}
)

const (
	testDraftPath     = "/home/user/.drudge/projects/demo/drafts/new-task.md"
	testTaskDraftPath = "/home/user/.drudge/projects/demo/drafts/3f9a1c2e-0000-0000-0000-000000000001.md"
)

var (
	errDraftUnwritable = errors.New("could not write the draft: permission denied")
	errEditorExit      = errors.New("exit status 1")
)

// fakeTaskService opens its draft at the test draft path or fails with
// openErr, and answers every create from the draft with createErr or a task
// titled Add retry. It opens the draft of every task at the test task draft
// path.
type fakeTaskService struct {
	openErr     error
	createErr   error
	createCount int
}

func (service *fakeTaskService) OpenDraft(projectSlug string) (string, error) {
	if service.openErr != nil {
		return "", service.openErr
	}
	return testDraftPath, nil
}

func (service *fakeTaskService) OpenTaskDraft(projectSlug string, id task.TaskID) (string, error) {
	return testTaskDraftPath, nil
}

func (service *fakeTaskService) CreateTaskFromDraft(projectSlug string) (*task.Task, error) {
	service.createCount++
	if service.createErr != nil {
		return nil, service.createErr
	}
	return &task.Task{ID: "3f9a1c2e-0000-4000-8000-000000000001", Title: "Add retry"}, nil
}

// fakeEditor records the path of every edit and exits with exitErr.
type fakeEditor struct {
	exitErr error
	paths   []string
}

func (editor *fakeEditor) Name() string {
	return "vi"
}

func (editor *fakeEditor) Edit(path string, done tea.ExecCallback) tea.Cmd {
	editor.paths = append(editor.paths, path)
	return func() tea.Msg { return done(editor.exitErr) }
}

// newDraftTestModel returns a model of one task list Tab whose new tasks go
// to service, whose edits from a draft go to drudgers and whose drafts are
// edited by editor.
func newDraftTestModel(t *testing.T, source *fakeTaskSource, service *fakeTaskService, drudgers *fakeCommandService, editor *fakeEditor) *Model {
	t.Helper()
	model := newTestModelWithSource(t, source, testTabs(t, "Tasks"), nil)
	services := actions.Services{Drudgers: drudgers, Tasks: service, Editor: editor}
	model.commands = actions.New(services, testProject.Slug, commandKinds)
	recordTimers(model)
	update(model, tea.WindowSizeMsg{Width: 140, Height: 24})
	startFeeds(model)
	return model
}

func TestModel_NewTask(t *testing.T) {
	cases := []struct {
		name            string
		service         *fakeTaskService
		editorErr       error
		wantEditedPaths []string
		wantCreateCount int
		wantLine        string
		wantIsRefresh   bool
	}{
		{
			name:            "an edited draft adds its task",
			service:         &fakeTaskService{},
			wantEditedPaths: []string{testDraftPath},
			wantCreateCount: 1,
			wantLine:        "✓ 3f9a1c2e  Add retry added",
			wantIsRefresh:   true,
		},
		{
			name:            "an unchanged draft skips",
			service:         &fakeTaskService{createErr: task.ErrDraftUnchanged},
			wantEditedPaths: []string{testDraftPath},
			wantCreateCount: 1,
			wantLine:        "· nothing changed",
			wantIsRefresh:   true,
		},
		{
			name:     "a draft that cannot open opens no editor",
			service:  &fakeTaskService{openErr: errDraftUnwritable},
			wantLine: "✗ " + errDraftUnwritable.Error(),
		},
		{
			name:            "a failed editor adds no task",
			service:         &fakeTaskService{},
			editorErr:       errEditorExit,
			wantEditedPaths: []string{testDraftPath},
			wantLine:        "✗ editor vi failed: exit status 1",
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			source := &fakeTaskSource{tasks: tuitest.ListedTasks()}
			editor := &fakeEditor{exitErr: testCase.editorErr}
			model := newDraftTestModel(t, source, testCase.service, &fakeCommandService{}, editor)
			listCountBefore := source.callCount.Load()

			pressKeys(model, []tea.Msg{keyNewTask})

			if !slices.Equal(editor.paths, testCase.wantEditedPaths) {
				t.Errorf("expected edits of %q, got %q", testCase.wantEditedPaths, editor.paths)
			}
			if testCase.service.createCount != testCase.wantCreateCount {
				t.Errorf("expected %d creates, got %d", testCase.wantCreateCount, testCase.service.createCount)
			}
			if got := commandLine(model); got != testCase.wantLine {
				t.Errorf("expected line %q, got %q", testCase.wantLine, got)
			}
			if gotIsRefresh := source.callCount.Load() > listCountBefore; gotIsRefresh != testCase.wantIsRefresh {
				t.Errorf("expected the tasks feed to refresh: %v, got %v", testCase.wantIsRefresh, gotIsRefresh)
			}
		})
	}
}

func TestModel_NewTaskRefusedWhileCommandRuns(t *testing.T) {
	editor := &fakeEditor{}
	model := newDraftTestModel(t, &fakeTaskSource{tasks: tuitest.ListedTasks()}, &fakeTaskService{}, &fakeCommandService{}, editor)
	startHeld(model, views.RunTaskMsg{TaskID: runTaskID})

	pressKeys(model, []tea.Msg{keyNewTask})

	if len(editor.paths) != 0 {
		t.Errorf("expected no editor while a command runs, got %q", editor.paths)
	}
	if got, want := commandLine(model), "! wait, still running run 7c1e4f90"; got != want {
		t.Errorf("expected line %q, got %q", want, got)
	}
}

func TestModel_EditTask(t *testing.T) {
	cursorTaskID := tuitest.ListedTasks()[0].Task.ID
	cases := []struct {
		name            string
		editorErr       error
		wantEditedPaths []string
		wantEditIDs     []task.TaskID
		wantLine        string
	}{
		{
			name:            "an edited draft edits the task under the cursor",
			wantEditedPaths: []string{testTaskDraftPath},
			wantEditIDs:     []task.TaskID{cursorTaskID},
			wantLine:        "✓ 3f9a1c2e  Add retry edited · title",
		},
		{
			name:            "a failed editor edits no task",
			editorErr:       errEditorExit,
			wantEditedPaths: []string{testTaskDraftPath},
			wantLine:        "✗ editor vi failed: exit status 1",
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			drudgers := &fakeCommandService{}
			editor := &fakeEditor{exitErr: testCase.editorErr}
			model := newDraftTestModel(t, &fakeTaskSource{tasks: tuitest.ListedTasks()}, &fakeTaskService{}, drudgers, editor)

			pressKeys(model, []tea.Msg{keyEditTask})

			if !slices.Equal(editor.paths, testCase.wantEditedPaths) {
				t.Errorf("expected edits of %q, got %q", testCase.wantEditedPaths, editor.paths)
			}
			if !slices.Equal(drudgers.editIDs, testCase.wantEditIDs) {
				t.Errorf("expected edits from a draft of %v, got %v", testCase.wantEditIDs, drudgers.editIDs)
			}
			if got := commandLine(model); got != testCase.wantLine {
				t.Errorf("expected line %q, got %q", testCase.wantLine, got)
			}
		})
	}
}
