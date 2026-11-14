package tui

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/IgorBolotnikov/DRUDGE/internal/task"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/actions"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/tuitest"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/views"
)

var keyNewTask = tea.KeyPressMsg{Code: 'n', Text: "n"}

var errUnknownBlocker = errors.New(`no task matches "a01f3b55"`)

const testTemplate = "---\ntitle: \nstatus: todo\nticket: \nparent: \nblocked-by: \nrepos: \npull-requests: \n---\n\n"

const validDraft = "---\ntitle: Add retry\nstatus: todo\n---\n\nRetry the upload.\n"

// fakeTaskService refuses every task with createErr and records the title of
// every task it is asked to add.
type fakeTaskService struct {
	createErr error
	titles    []string
}

func (service *fakeTaskService) CreateTask(dto task.CreateTaskDto) (*task.Task, error) {
	service.titles = append(service.titles, dto.Title)
	if service.createErr != nil {
		return nil, service.createErr
	}
	return &task.Task{ID: "3f9a1c2e-0000-4000-8000-000000000001", Title: dto.Title}, nil
}

// fakeEditor records what the draft held when each edit started and writes
// the next of its saves into it. An edit with no save left leaves the draft
// as it is.
type fakeEditor struct {
	saves  []string
	opened []string
}

func (editor *fakeEditor) edit(path string) tea.Cmd {
	return func() tea.Msg {
		content, err := os.ReadFile(path)
		if err != nil {
			return draftEditedMsg{err: err}
		}
		editor.opened = append(editor.opened, string(content))
		if len(editor.saves) > 0 {
			if err := os.WriteFile(path, []byte(editor.saves[0]), draftFilePerm); err != nil {
				return draftEditedMsg{err: err}
			}
			editor.saves = editor.saves[1:]
		}
		return draftEditedMsg{}
	}
}

// newDraftTestModel returns a model of one task list Tab whose new tasks go
// to service and whose drafts are written by editor in a temp directory.
func newDraftTestModel(t *testing.T, source *fakeTaskSource, service *fakeTaskService, editor *fakeEditor) *Model {
	t.Helper()
	model := newTestModelWithSource(t, source, testTabs(t, "Tasks"), nil)
	model.commands = actions.New(actions.Services{Drudgers: &fakeCommandService{}, Tasks: service}, testProject.Slug, commandKinds)
	model.draft = newDraftFile(testProject.Slug, task.StatusTodo)
	model.draft.path = filepath.Join(t.TempDir(), "draft.md")
	model.runEditor = editor.edit
	recordTimers(model)
	update(model, tea.WindowSizeMsg{Width: 140, Height: 24})
	startFeeds(model)
	return model
}

func TestModel_NewTask(t *testing.T) {
	cases := []struct {
		name          string
		createErr     error
		saves         []string
		wantTitles    []string
		wantLine      string
		wantDraft     string
		wantIsRefresh bool
	}{
		{name: "a draft adds its task", saves: []string{validDraft}, wantTitles: []string{"Add retry"}, wantLine: "✓ 3f9a1c2e  Add retry added", wantIsRefresh: true},
		{name: "an unchanged draft cancels", saves: []string{testTemplate}, wantLine: "· nothing changed"},
		{name: "a draft left alone cancels", wantLine: "· nothing changed"},
		{name: "a template the editor trimmed cancels", saves: []string{"---\ntitle:\nstatus: todo\nticket:\nparent:\nblocked-by:\nrepos:\npull-requests:\n---\n"}, wantLine: "· nothing changed"},
		{name: "a draft with no title and no body cancels", saves: []string{"---\nstatus: draft\n---\n"}, wantLine: "· nothing changed"},
		{name: "an emptied draft cancels", saves: []string{""}, wantLine: "· nothing changed"},
		{
			name:      "a draft that does not parse stays",
			saves:     []string{"---\ntilte: Add retry\n---\n"},
			wantLine:  `✗ line 2: a draft has no key "tilte", use one of: title, status, ticket, parent, blocked-by, repos, pull-requests · n to reopen your draft`,
			wantDraft: "---\ntilte: Add retry\n---\n",
		},
		{
			name:          "a draft the service refuses stays",
			createErr:     errUnknownBlocker,
			saves:         []string{validDraft},
			wantTitles:    []string{"Add retry"},
			wantLine:      "✗ " + errUnknownBlocker.Error() + " · n to reopen your draft",
			wantDraft:     validDraft,
			wantIsRefresh: true,
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			source := &fakeTaskSource{tasks: tuitest.ListedTasks()}
			service := &fakeTaskService{createErr: testCase.createErr}
			editor := &fakeEditor{saves: testCase.saves}
			model := newDraftTestModel(t, source, service, editor)
			listCountBefore := source.callCount.Load()

			pressKeys(model, []tea.Msg{keyNewTask})

			if !slices.Equal(editor.opened, []string{testTemplate}) {
				t.Errorf("expected the editor to open the template once, got %q", editor.opened)
			}
			if !slices.Equal(service.titles, testCase.wantTitles) {
				t.Errorf("expected tasks %q added, got %q", testCase.wantTitles, service.titles)
			}
			if got := commandLine(model); got != testCase.wantLine {
				t.Errorf("expected line %q, got %q", testCase.wantLine, got)
			}
			content, err := os.ReadFile(model.draft.path)
			switch {
			case testCase.wantDraft == "" && !errors.Is(err, os.ErrNotExist):
				t.Errorf("expected the draft removed, got %q, %v", content, err)
			case testCase.wantDraft != "" && string(content) != testCase.wantDraft:
				t.Errorf("expected the draft kept as %q, got %q, %v", testCase.wantDraft, content, err)
			}
			if gotIsRefresh := source.callCount.Load() > listCountBefore; gotIsRefresh != testCase.wantIsRefresh {
				t.Errorf("expected the tasks feed to refresh: %v, got %v", testCase.wantIsRefresh, gotIsRefresh)
			}
		})
	}
}

func TestModel_NewTaskReopensKeptDraft(t *testing.T) {
	brokenDraft := "---\ntitle Add retry\n---\n"
	service := &fakeTaskService{}
	editor := &fakeEditor{saves: []string{brokenDraft, validDraft}}
	model := newDraftTestModel(t, &fakeTaskSource{tasks: tuitest.ListedTasks()}, service, editor)

	pressKeys(model, []tea.Msg{keyNewTask, keyNewTask})

	if wantOpened := []string{testTemplate, brokenDraft}; !slices.Equal(editor.opened, wantOpened) {
		t.Errorf("expected the editor to open %q, got %q", wantOpened, editor.opened)
	}
	if wantTitles := []string{"Add retry"}; !slices.Equal(service.titles, wantTitles) {
		t.Errorf("expected tasks %q added, got %q", wantTitles, service.titles)
	}
	if _, err := os.Stat(model.draft.path); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("expected the draft removed after the task was added, got %v", err)
	}
}

func TestModel_NewTaskRefusedWhileCommandRuns(t *testing.T) {
	editor := &fakeEditor{}
	model := newDraftTestModel(t, &fakeTaskSource{tasks: tuitest.ListedTasks()}, &fakeTaskService{}, editor)
	startHeld(model, views.RunTaskMsg{TaskID: runTaskID})

	pressKeys(model, []tea.Msg{keyNewTask})

	if len(editor.opened) != 0 {
		t.Errorf("expected no editor while a command runs, got %q", editor.opened)
	}
	if got, want := commandLine(model), "! wait, still running run 7c1e4f90"; got != want {
		t.Errorf("expected line %q, got %q", want, got)
	}
}
