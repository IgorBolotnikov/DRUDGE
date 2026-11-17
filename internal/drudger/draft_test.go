package drudger

import (
	"errors"
	"reflect"
	"slices"
	"testing"

	"github.com/IgorBolotnikov/DRUDGE/internal/task"
)

var errDraftRemoval = errors.New("could not remove the draft: permission denied")

// fakeDraftRepo keeps the drafts of one project in memory. removeErr fails
// every removal.
type fakeDraftRepo struct {
	drafts    map[string]string
	removeErr error
}

func (repo *fakeDraftRepo) ReadDraft(projectSlug string, name string) (string, bool, error) {
	content, isFound := repo.drafts[name]
	return content, isFound, nil
}

func (repo *fakeDraftRepo) WriteDraft(projectSlug string, name string, content string) error {
	repo.drafts[name] = content
	return nil
}

func (repo *fakeDraftRepo) RemoveDraft(projectSlug string, name string) error {
	if repo.removeErr != nil {
		return repo.removeErr
	}
	delete(repo.drafts, name)
	return nil
}

func (repo *fakeDraftRepo) DraftPath(projectSlug string, name string) (string, error) {
	return "/home/user/.drudge/projects/demo/drafts/" + name + ".md", nil
}

func TestDrudgerService_EditTaskFromDraft(t *testing.T) {
	const (
		unchangedDraft = "---\ntitle: Fix login\nstatus: todo\n---\n\nSSO is broken\n"
		titleDraft     = "---\ntitle: Fix SSO login\nstatus: todo\n---\n\nSSO is broken\n"
		brokenDraft    = "---\ntilte: Fix SSO login\n---\n"
		managedDraft   = "---\ntitle: Fix login\nstatus: in-progress\n---\n\nSSO is broken\n"
	)
	cases := []struct {
		name       string
		draft      string
		removeErr  error
		wantErr    error
		wantErrMsg string
		wantFields []string
		wantTitle  string
		wantStatus task.TaskStatus
		wantDraft  string
		// wantIsWarned says the failed removal is reported as a warning.
		wantIsWarned bool
	}{
		{
			name:       "a fresh draft left as it was is unchanged",
			draft:      unchangedDraft,
			wantErr:    task.ErrDraftUnchanged,
			wantTitle:  "Fix login",
			wantStatus: task.StatusTodo,
		},
		{
			name:       "a draft the editor removed is unchanged",
			wantErr:    task.ErrDraftUnchanged,
			wantTitle:  "Fix login",
			wantStatus: task.StatusTodo,
		},
		{
			name:       "an unchanged draft that cannot be removed fails",
			draft:      unchangedDraft,
			removeErr:  errDraftRemoval,
			wantErr:    errDraftRemoval,
			wantTitle:  "Fix login",
			wantStatus: task.StatusTodo,
			wantDraft:  unchangedDraft,
		},
		{
			name:       "a draft that does not parse stays",
			draft:      brokenDraft,
			wantErrMsg: `line 2: a draft has no key "tilte", use one of: title, status, ticket, parent, blocked-by, repos, pull-requests`,
			wantTitle:  "Fix login",
			wantStatus: task.StatusTodo,
			wantDraft:  brokenDraft,
		},
		{
			name:       "a status drudge writes itself is refused and the draft stays",
			draft:      managedDraft,
			wantErrMsg: `status "in-progress" is one drudge writes itself when a Session starts and ends, force the edit to set it by hand`,
			wantTitle:  "Fix login",
			wantStatus: task.StatusTodo,
			wantDraft:  managedDraft,
		},
		{
			name:       "an edit removes the draft",
			draft:      titleDraft,
			wantFields: []string{"title"},
			wantTitle:  "Fix SSO login",
			wantStatus: task.StatusTodo,
		},
		{
			name:         "a draft that cannot be removed after the edit is a warning",
			draft:        titleDraft,
			removeErr:    errDraftRemoval,
			wantFields:   []string{"title"},
			wantTitle:    "Fix SSO login",
			wantStatus:   task.StatusTodo,
			wantDraft:    titleDraft,
			wantIsWarned: true,
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			stored := todoTask()
			service := newTestService(stored)
			drafts := &fakeDraftRepo{drafts: map[string]string{}, removeErr: testCase.removeErr}
			if testCase.draft != "" {
				drafts.drafts[string(stored.ID)] = testCase.draft
			}
			service.tasks = task.NewTaskService(service.taskRepo, drafts, noopTaskProgress{}, task.StatusDraft, nil)

			edited, fields, err := service.EditTaskFromDraft(testProjectSlug, stored.ID)

			switch {
			case testCase.wantErrMsg != "":
				if err == nil || err.Error() != testCase.wantErrMsg {
					t.Errorf("expected error %q, got %v", testCase.wantErrMsg, err)
				}
			case !errors.Is(err, testCase.wantErr):
				t.Errorf("expected error %v, got %v", testCase.wantErr, err)
			}
			if !slices.Equal(fields, testCase.wantFields) {
				t.Errorf("expected fields %q, got %q", testCase.wantFields, fields)
			}
			if wantIsReturned := len(testCase.wantFields) > 0; wantIsReturned != (edited != nil) {
				t.Errorf("expected the edited task back: %v, got %+v", wantIsReturned, edited)
			}
			if stored.Title != testCase.wantTitle || stored.Status != testCase.wantStatus {
				t.Errorf("expected task %q %q, got %q %q", testCase.wantTitle, testCase.wantStatus, stored.Title, stored.Status)
			}
			if got := drafts.drafts[string(stored.ID)]; got != testCase.wantDraft {
				t.Errorf("expected draft %q, got %q", testCase.wantDraft, got)
			}
			var wantWarnings []task.EditDraftRemovalFailed
			if testCase.wantIsWarned {
				wantWarnings = []task.EditDraftRemovalFailed{{Task: edited, Err: errDraftRemoval}}
			}
			if got := reportedEvents[task.EditDraftRemovalFailed](service.progress); !reflect.DeepEqual(got, wantWarnings) {
				t.Errorf("expected warnings %+v, got %+v", wantWarnings, got)
			}
		})
	}
}
