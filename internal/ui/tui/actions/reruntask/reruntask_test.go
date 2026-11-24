package reruntask_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/IgorBolotnikov/DRUDGE/internal/drudger"
	"github.com/IgorBolotnikov/DRUDGE/internal/task"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/actions"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/actions/reruntask"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/views"
)

const testProjectSlug = "uploader"

const testTaskID task.TaskID = "3f9a1c2e-0000-4000-8000-000000000001"

var errStillWorking = errors.New("task 3f9a1c2e has an agent still working on it")

// fakeDrudgerService refuses every rerun with rerunErr. It records the task of
// every rerun.
type fakeDrudgerService struct {
	drudgers []*drudger.Drudger
	rerunErr error
	rerunIDs []task.TaskID
}

func (service *fakeDrudgerService) RunTask(projectSlug string, requestedID task.TaskID, isDryRun bool) error {
	return errors.New("a rerun runs no task")
}

func (service *fakeDrudgerService) RerunTask(projectSlug string, requestedID task.TaskID, isDryRun bool) error {
	if projectSlug != testProjectSlug || isDryRun {
		return errors.New("the rerun went to another project or was a dry run")
	}
	service.rerunIDs = append(service.rerunIDs, requestedID)
	return service.rerunErr
}

func (service *fakeDrudgerService) MarkDone(projectSlug string, id task.TaskID) (*task.Task, []*task.Task, error) {
	return nil, nil, errors.New("a rerun marks no task done")
}

func (service *fakeDrudgerService) EditTask(projectSlug string, id task.TaskID, changes task.EditTaskDto) (*task.Task, error) {
	return nil, errors.New("this test edits no task")
}

func (service *fakeDrudgerService) EditTaskFromDraft(projectSlug string, id task.TaskID) (*task.Task, []string, error) {
	return nil, nil, errors.New("a rerun edits no task")
}

func (service *fakeDrudgerService) RetryPullRequests(projectSlug string, requestedID task.TaskID) (drudger.PullRequestsRetried, error) {
	return drudger.PullRequestsRetried{}, errors.New("a rerun opens no pull request")
}

func (service *fakeDrudgerService) ReclaimDrudgers(projectSlug string) ([]drudger.FreedSlot, error) {
	return nil, errors.New("a rerun reclaims no slot")
}

func (service *fakeDrudgerService) NukeDrudger(projectSlug string, slot int, isForced bool) error {
	return errors.New("a rerun nukes no Drudger")
}

func (service *fakeDrudgerService) ReadDrudgers(projectSlug string) ([]*drudger.Drudger, error) {
	return service.drudgers, nil
}

func TestKind(t *testing.T) {
	holders := []*drudger.Drudger{{Slot: 1, TaskID: "another"}, {Slot: 3, TaskID: testTaskID}}
	rerunFeeds := []views.FeedName{views.TasksFeed, views.DrudgersFeed}
	cases := []struct {
		name        string
		service     *fakeDrudgerService
		wantVerdict string
		wantErr     error
	}{
		{name: "rerun names the Drudger that took the task", service: &fakeDrudgerService{drudgers: holders}, wantVerdict: "3f9a1c2e handed to Drudger 3"},
		{name: "refused rerun carries the error of the service", service: &fakeDrudgerService{rerunErr: errStillWorking}, wantErr: errStillWorking},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			commands := actions.New(actions.Services{Drudgers: testCase.service}, testProjectSlug, []actions.Kind{reruntask.Kind})
			_, cmd := commands.Start(views.RerunTaskMsg{TaskID: testTaskID})
			done, ok := cmd().(actions.DoneMsg)
			if !ok {
				t.Fatal("expected a done message")
			}
			if !slices.Equal(testCase.service.rerunIDs, []task.TaskID{testTaskID}) {
				t.Errorf("expected one rerun of %s, got %v", testTaskID, testCase.service.rerunIDs)
			}
			wantCommand := actions.Command{Name: views.RerunTaskCommand, Target: "3f9a1c2e"}
			if done.Command != wantCommand {
				t.Errorf("expected done command %v, got %v", wantCommand, done.Command)
			}
			if done.Verdict != testCase.wantVerdict {
				t.Errorf("expected verdict %q, got %q", testCase.wantVerdict, done.Verdict)
			}
			if !errors.Is(done.Err, testCase.wantErr) {
				t.Errorf("expected error %v, got %v", testCase.wantErr, done.Err)
			}
			if !slices.Equal(done.Feeds, rerunFeeds) {
				t.Errorf("expected feeds %v, got %v", rerunFeeds, done.Feeds)
			}
		})
	}
}

func TestKind_Question(t *testing.T) {
	commands := actions.New(actions.Services{}, testProjectSlug, []actions.Kind{reruntask.Kind})

	question, ok := commands.Question(views.RerunTaskMsg{TaskID: testTaskID})

	want := "Start 3f9a1c2e over? What its last run left behind is cleared."
	if !ok || question != want {
		t.Errorf("expected question %q, got %q (ok %v)", want, question, ok)
	}
}
