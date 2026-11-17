package newtask_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/IgorBolotnikov/DRUDGE/internal/task"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/actions"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/actions/newtask"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/views"
)

const testProjectSlug = "uploader"

const testTaskID task.TaskID = "3f9a1c2e-0000-4000-8000-000000000001"

var errUnknownBlocker = errors.New(`no task matches "a01f3b55"`)

// fakeTaskService answers every create from a draft with createErr, or with a
// task titled Add retry, and records the project of every create.
type fakeTaskService struct {
	createErr    error
	projectSlugs []string
}

func (service *fakeTaskService) OpenDraft(projectSlug string) (string, error) {
	return "", errors.New("the kind opens no draft")
}

func (service *fakeTaskService) CreateTaskFromDraft(projectSlug string) (*task.Task, error) {
	service.projectSlugs = append(service.projectSlugs, projectSlug)
	if service.createErr != nil {
		return nil, service.createErr
	}
	return &task.Task{ID: testTaskID, Title: "Add retry"}, nil
}

func TestKind(t *testing.T) {
	cases := []struct {
		name          string
		createErr     error
		wantVerdict   string
		wantIsSkipped bool
		wantErr       error
		wantErrText   string
	}{
		{name: "create names the new task", wantVerdict: "3f9a1c2e  Add retry added"},
		{name: "unchanged draft skips", createErr: task.ErrDraftUnchanged, wantVerdict: "nothing changed", wantIsSkipped: true},
		{
			name:        "refused create points at the kept draft",
			createErr:   errUnknownBlocker,
			wantErr:     errUnknownBlocker,
			wantErrText: errUnknownBlocker.Error() + " · n to reopen your draft",
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			service := &fakeTaskService{createErr: testCase.createErr}
			commands := actions.New(actions.Services{Tasks: service}, testProjectSlug, []actions.Kind{newtask.Kind})
			_, cmd := commands.Start(views.NewTaskMsg{})
			done, ok := cmd().(actions.DoneMsg)
			if !ok {
				t.Fatal("expected a done message")
			}
			if wantProjectSlugs := []string{testProjectSlug}; !slices.Equal(service.projectSlugs, wantProjectSlugs) {
				t.Errorf("expected one create in %v, got %v", wantProjectSlugs, service.projectSlugs)
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
			if wantFeeds := []views.FeedName{views.TasksFeed}; !slices.Equal(done.Feeds, wantFeeds) {
				t.Errorf("expected feeds %v, got %v", wantFeeds, done.Feeds)
			}
		})
	}
}
