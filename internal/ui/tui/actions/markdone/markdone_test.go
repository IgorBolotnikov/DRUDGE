package markdone_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/IgorBolotnikov/DRUDGE/internal/drudger"
	"github.com/IgorBolotnikov/DRUDGE/internal/task"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/actions"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/actions/markdone"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/views"
)

const testProjectSlug = "uploader"

const testTaskID task.TaskID = "3f9a1c2e-0000-4000-8000-000000000001"

var errNotUnmerged = errors.New(`task 3f9a1c2e is "todo", only a task that is "unmerged" can be marked done`)

// fakeDrudgerService refuses every mark done with doneErr. A mark done
// unblocks the unblocked tasks. It records the task of every mark done.
type fakeDrudgerService struct {
	unblocked []*task.Task
	doneErr   error
	doneIDs   []task.TaskID
}

func (service *fakeDrudgerService) RunTask(projectSlug string, requestedID task.TaskID, isDryRun bool) error {
	return errors.New("a mark done runs no task")
}

func (service *fakeDrudgerService) MarkDone(projectSlug string, id task.TaskID) (*task.Task, []*task.Task, error) {
	if projectSlug != testProjectSlug {
		return nil, nil, errors.New("the mark done went to another project")
	}
	service.doneIDs = append(service.doneIDs, id)
	if service.doneErr != nil {
		return nil, nil, service.doneErr
	}
	return &task.Task{ID: id, Status: task.StatusDone}, service.unblocked, nil
}

func (service *fakeDrudgerService) EditTaskFromDraft(projectSlug string, id task.TaskID) (*task.Task, []string, error) {
	return nil, nil, errors.New("a mark done edits no task")
}

func (service *fakeDrudgerService) ReadDrudgers(projectSlug string) ([]*drudger.Drudger, error) {
	return nil, errors.New("a mark done reads no Drudgers")
}

func TestKind(t *testing.T) {
	oneUnblocked := []*task.Task{{ID: "4d5e6f70-0000-4000-8000-000000000002"}}
	twoUnblocked := []*task.Task{{ID: "4d5e6f70-0000-4000-8000-000000000002"}, {ID: "7a8b9c0d-0000-4000-8000-000000000003"}}
	doneFeeds := []views.FeedName{views.TasksFeed}
	cases := []struct {
		name        string
		service     *fakeDrudgerService
		wantVerdict string
		wantErr     error
	}{
		{name: "done with no task unblocked", service: &fakeDrudgerService{}, wantVerdict: "3f9a1c2e done"},
		{name: "done with one task unblocked", service: &fakeDrudgerService{unblocked: oneUnblocked}, wantVerdict: "3f9a1c2e done · 1 task runnable: 4d5e6f70"},
		{name: "done with two tasks unblocked", service: &fakeDrudgerService{unblocked: twoUnblocked}, wantVerdict: "3f9a1c2e done · 2 tasks runnable: 4d5e6f70, 7a8b9c0d"},
		{name: "refused done carries the error of the service", service: &fakeDrudgerService{doneErr: errNotUnmerged}, wantErr: errNotUnmerged},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			commands := actions.New(actions.Services{Drudgers: testCase.service}, testProjectSlug, []actions.Kind{markdone.Kind})
			_, cmd := commands.Start(views.MarkDoneMsg{TaskID: testTaskID})
			done, ok := cmd().(actions.DoneMsg)
			if !ok {
				t.Fatal("expected a done message")
			}
			if !slices.Equal(testCase.service.doneIDs, []task.TaskID{testTaskID}) {
				t.Errorf("expected one mark done of %s, got %v", testTaskID, testCase.service.doneIDs)
			}
			wantCommand := actions.Command{Name: views.MarkDoneCommand, Target: "3f9a1c2e"}
			if done.Command != wantCommand {
				t.Errorf("expected done command %v, got %v", wantCommand, done.Command)
			}
			if done.Verdict != testCase.wantVerdict {
				t.Errorf("expected verdict %q, got %q", testCase.wantVerdict, done.Verdict)
			}
			if !errors.Is(done.Err, testCase.wantErr) {
				t.Errorf("expected error %v, got %v", testCase.wantErr, done.Err)
			}
			if !slices.Equal(done.Feeds, doneFeeds) {
				t.Errorf("expected feeds %v, got %v", doneFeeds, done.Feeds)
			}
		})
	}
}
