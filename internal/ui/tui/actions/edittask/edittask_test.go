package edittask_test

import (
	"errors"
	"slices"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/IgorBolotnikov/DRUDGE/internal/drudger"
	"github.com/IgorBolotnikov/DRUDGE/internal/task"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/actions"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/actions/edittask"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/views"
)

const (
	testProjectSlug             = "uploader"
	testTaskID      task.TaskID = "3f9a1c2e-0000-4000-8000-000000000001"
	testDraftPath               = "/home/user/.drudge/projects/uploader/drafts/3f9a1c2e-0000-4000-8000-000000000001.md"
)

var (
	errManagedStatus   = errors.New(`status "in-progress" is one drudge writes itself when a Session starts and ends, force the edit to set it by hand`)
	errDraftUnwritable = errors.New("could not write the draft: permission denied")
	errEditorExit      = errors.New("exit status 1")
)

// fakeTaskService opens the draft of a task at the test draft path or fails
// with openErr. It records the task of every open.
type fakeTaskService struct {
	openErr error
	openIDs []task.TaskID
}

func (service *fakeTaskService) OpenDraft(projectSlug string) (string, error) {
	return "", errors.New("an edit opens no draft of a new task")
}

func (service *fakeTaskService) OpenTaskDraft(projectSlug string, id task.TaskID) (string, error) {
	service.openIDs = append(service.openIDs, id)
	if service.openErr != nil {
		return "", service.openErr
	}
	return testDraftPath, nil
}

func (service *fakeTaskService) CreateTaskFromDraft(projectSlug string) (*task.Task, error) {
	return nil, errors.New("an edit adds no task")
}

// fakeDrudgerService answers every edit from a draft with editErr, or with
// the task titled Add retry and a changed title and status. It records the
// task of every edit.
type fakeDrudgerService struct {
	editErr error
	editIDs []task.TaskID
}

func (service *fakeDrudgerService) RunTask(projectSlug string, requestedID task.TaskID, isDryRun bool) error {
	return errors.New("an edit runs no task")
}

func (service *fakeDrudgerService) RerunTask(projectSlug string, requestedID task.TaskID, isDryRun bool) error {
	return errors.New("an edit reruns no task")
}

func (service *fakeDrudgerService) MarkDone(projectSlug string, id task.TaskID) (*task.Task, []*task.Task, error) {
	return nil, nil, errors.New("an edit marks no task done")
}

func (service *fakeDrudgerService) EditTaskFromDraft(projectSlug string, id task.TaskID) (*task.Task, []string, error) {
	if projectSlug != testProjectSlug {
		return nil, nil, errors.New("the edit went to another project")
	}
	service.editIDs = append(service.editIDs, id)
	if service.editErr != nil {
		return nil, nil, service.editErr
	}
	return &task.Task{ID: id, Title: "Add retry"}, []string{"title", "status"}, nil
}

func (service *fakeDrudgerService) ReclaimDrudgers(projectSlug string) ([]drudger.FreedSlot, error) {
	return nil, errors.New("an edit reclaims no slot")
}

func (service *fakeDrudgerService) ReadDrudgers(projectSlug string) ([]*drudger.Drudger, error) {
	return nil, errors.New("an edit reads no Drudgers")
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
	editFeeds := []views.FeedName{views.TasksFeed}
	cases := []struct {
		name            string
		tasks           *fakeTaskService
		drudgers        *fakeDrudgerService
		editorErr       error
		wantEditedPaths []string
		wantEditIDs     []task.TaskID
		wantVerdict     string
		wantIsSkipped   bool
		wantErr         error
		wantErrText     string
		wantFeeds       []views.FeedName
	}{
		{
			name:            "edit names the task and the changed fields",
			tasks:           &fakeTaskService{},
			drudgers:        &fakeDrudgerService{},
			wantEditedPaths: []string{testDraftPath},
			wantEditIDs:     []task.TaskID{testTaskID},
			wantVerdict:     "3f9a1c2e  Add retry edited · title, status",
			wantFeeds:       editFeeds,
		},
		{
			name:            "unchanged draft skips",
			tasks:           &fakeTaskService{},
			drudgers:        &fakeDrudgerService{editErr: task.ErrDraftUnchanged},
			wantEditedPaths: []string{testDraftPath},
			wantEditIDs:     []task.TaskID{testTaskID},
			wantVerdict:     "nothing changed",
			wantIsSkipped:   true,
			wantFeeds:       editFeeds,
		},
		{
			name:            "refused edit points at the kept draft",
			tasks:           &fakeTaskService{},
			drudgers:        &fakeDrudgerService{editErr: errManagedStatus},
			wantEditedPaths: []string{testDraftPath},
			wantEditIDs:     []task.TaskID{testTaskID},
			wantErr:         errManagedStatus,
			wantErrText:     errManagedStatus.Error() + " · e to reopen your draft",
			wantFeeds:       editFeeds,
		},
		{
			name:        "draft that cannot open opens no editor",
			tasks:       &fakeTaskService{openErr: errDraftUnwritable},
			drudgers:    &fakeDrudgerService{},
			wantErr:     errDraftUnwritable,
			wantErrText: errDraftUnwritable.Error(),
		},
		{
			name:            "failed editor edits no task",
			tasks:           &fakeTaskService{},
			drudgers:        &fakeDrudgerService{},
			editorErr:       errEditorExit,
			wantEditedPaths: []string{testDraftPath},
			wantErr:         errEditorExit,
			wantErrText:     "editor vi failed: exit status 1",
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			editor := &fakeEditor{exitErr: testCase.editorErr}
			services := actions.Services{Drudgers: testCase.drudgers, Tasks: testCase.tasks, Editor: editor}
			commands := actions.New(services, testProjectSlug, []actions.Kind{edittask.Kind})
			_, cmd := commands.Start(views.EditTaskMsg{TaskID: testTaskID})
			done := finish(t, cmd())

			if !slices.Equal(testCase.tasks.openIDs, []task.TaskID{testTaskID}) {
				t.Errorf("expected the draft of %s opened, got %v", testTaskID, testCase.tasks.openIDs)
			}
			if !slices.Equal(editor.paths, testCase.wantEditedPaths) {
				t.Errorf("expected edits of %q, got %q", testCase.wantEditedPaths, editor.paths)
			}
			if !slices.Equal(testCase.drudgers.editIDs, testCase.wantEditIDs) {
				t.Errorf("expected edits from a draft of %v, got %v", testCase.wantEditIDs, testCase.drudgers.editIDs)
			}
			wantCommand := actions.Command{Name: views.EditTaskCommand, Target: "3f9a1c2e"}
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

func (service *fakeTaskService) RemoveTask(projectSlug string, id task.TaskID, isForced bool, sessions task.SessionKeeper, confirm task.ConfirmRemoval) error {
	return errors.New("this command removes no task")
}
