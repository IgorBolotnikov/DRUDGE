package newtask_test

import (
	"errors"
	"slices"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/IgorBolotnikov/DRUDGE/internal/task"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/actions"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/actions/newtask"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/views"
)

const (
	testProjectSlug             = "uploader"
	testTaskID      task.TaskID = "3f9a1c2e-0000-4000-8000-000000000001"
	testDraftPath               = "/home/user/.drudge/projects/uploader/drafts/new-task.md"
)

var (
	errUnknownBlocker  = errors.New(`no task matches "a01f3b55"`)
	errDraftUnwritable = errors.New("could not write the draft: permission denied")
	errEditorExit      = errors.New("exit status 1")
)

// fakeTaskService opens its draft at the test draft path or fails with
// openErr. It answers every create from a draft with createErr, or with a task
// titled Add retry, and records the project of every create.
type fakeTaskService struct {
	openErr      error
	createErr    error
	projectSlugs []string
}

func (service *fakeTaskService) OpenDraft(projectSlug string) (string, error) {
	if service.openErr != nil {
		return "", service.openErr
	}
	return testDraftPath, nil
}

func (service *fakeTaskService) OpenTaskDraft(projectSlug string, id task.TaskID) (string, error) {
	return "", errors.New("a new task opens no draft of a task")
}

func (service *fakeTaskService) CreateTaskFromDraft(projectSlug string) (*task.Task, error) {
	service.projectSlugs = append(service.projectSlugs, projectSlug)
	if service.createErr != nil {
		return nil, service.createErr
	}
	return &task.Task{ID: testTaskID, Title: "Add retry"}, nil
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

// finish runs the message of a command to its DoneMsg the way the program
// does: it runs the commands of a batch.
func finish(t *testing.T, msg tea.Msg) actions.DoneMsg {
	t.Helper()
	if batch, ok := msg.(tea.BatchMsg); ok {
		if len(batch) != 1 {
			t.Fatalf("expected a batch of one command, got %d", len(batch))
		}
		return finish(t, batch[0]())
	}
	done, ok := msg.(actions.DoneMsg)
	if !ok {
		t.Fatalf("expected a done message, got %T", msg)
	}
	return done
}

func TestKind(t *testing.T) {
	createFeeds := []views.FeedName{views.TasksFeed}
	cases := []struct {
		name             string
		service          *fakeTaskService
		editorErr        error
		wantEditedPaths  []string
		wantProjectSlugs []string
		wantVerdict      string
		wantIsSkipped    bool
		wantErr          error
		wantErrText      string
		wantFeeds        []views.FeedName
	}{
		{
			name:             "create names the new task",
			service:          &fakeTaskService{},
			wantEditedPaths:  []string{testDraftPath},
			wantProjectSlugs: []string{testProjectSlug},
			wantVerdict:      "3f9a1c2e  Add retry added",
			wantFeeds:        createFeeds,
		},
		{
			name:             "unchanged draft skips",
			service:          &fakeTaskService{createErr: task.ErrDraftUnchanged},
			wantEditedPaths:  []string{testDraftPath},
			wantProjectSlugs: []string{testProjectSlug},
			wantVerdict:      "nothing changed",
			wantIsSkipped:    true,
			wantFeeds:        createFeeds,
		},
		{
			name:             "refused create points at the kept draft",
			service:          &fakeTaskService{createErr: errUnknownBlocker},
			wantEditedPaths:  []string{testDraftPath},
			wantProjectSlugs: []string{testProjectSlug},
			wantErr:          errUnknownBlocker,
			wantErrText:      errUnknownBlocker.Error() + " · n to reopen your draft",
			wantFeeds:        createFeeds,
		},
		{
			name:        "draft that cannot open opens no editor",
			service:     &fakeTaskService{openErr: errDraftUnwritable},
			wantErr:     errDraftUnwritable,
			wantErrText: errDraftUnwritable.Error(),
		},
		{
			name:            "failed editor adds no task",
			service:         &fakeTaskService{},
			editorErr:       errEditorExit,
			wantEditedPaths: []string{testDraftPath},
			wantErr:         errEditorExit,
			wantErrText:     "editor vi failed: exit status 1",
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			editor := &fakeEditor{exitErr: testCase.editorErr}
			services := actions.Services{Tasks: testCase.service, Editor: editor}
			commands := actions.New(services, testProjectSlug, []actions.Kind{newtask.Kind})
			_, cmd := commands.Start(views.NewTaskMsg{})
			done := finish(t, cmd())

			if !slices.Equal(editor.paths, testCase.wantEditedPaths) {
				t.Errorf("expected edits of %q, got %q", testCase.wantEditedPaths, editor.paths)
			}
			if !slices.Equal(testCase.service.projectSlugs, testCase.wantProjectSlugs) {
				t.Errorf("expected creates in %v, got %v", testCase.wantProjectSlugs, testCase.service.projectSlugs)
			}
			wantCommand := actions.Command{Name: views.NewTaskCommand, Target: "task"}
			if done.Command != wantCommand {
				t.Errorf("expected done command %v, got %v", wantCommand, done.Command)
			}
			if done.Verdict != testCase.wantVerdict || done.IsSkipped != testCase.wantIsSkipped {
				t.Errorf("expected verdict %q, skipped %v, got %q, skipped %v", testCase.wantVerdict, testCase.wantIsSkipped, done.Verdict, done.IsSkipped)
			}
			if !errors.Is(done.Err, testCase.wantErr) {
				t.Errorf("expected error %v, got %v", testCase.wantErr, done.Err)
			}
			if done.Err != nil && done.Err.Error() != testCase.wantErrText {
				t.Errorf("expected error text %q, got %q", testCase.wantErrText, done.Err.Error())
			}
			if !slices.Equal(done.Feeds, testCase.wantFeeds) {
				t.Errorf("expected feeds %v, got %v", testCase.wantFeeds, done.Feeds)
			}
		})
	}
}
