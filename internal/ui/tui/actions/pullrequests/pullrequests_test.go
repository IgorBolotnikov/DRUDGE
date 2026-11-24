package pullrequests_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/IgorBolotnikov/DRUDGE/internal/drudger"
	"github.com/IgorBolotnikov/DRUDGE/internal/task"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/actions"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/actions/pullrequests"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/views"
)

const testProjectSlug = "uploader"

const testTaskID task.TaskID = "3f9a1c2e-0000-4000-8000-000000000001"

var errStillWorking = errors.New("task 3f9a1c2e is in progress and its agent is still working, wait for it to finish")

// fakeDrudgerService returns retried from every retry, or refuses it with
// retryErr. It records the task of every retry.
type fakeDrudgerService struct {
	retried  drudger.PullRequestsRetried
	retryErr error
	retryIDs []task.TaskID
}

func (service *fakeDrudgerService) RunTask(projectSlug string, requestedID task.TaskID, isDryRun bool) error {
	return errors.New("a pull request retry runs no task")
}

func (service *fakeDrudgerService) RerunTask(projectSlug string, requestedID task.TaskID, isDryRun bool) error {
	return errors.New("a pull request retry reruns no task")
}

func (service *fakeDrudgerService) MarkDone(projectSlug string, id task.TaskID) (*task.Task, []*task.Task, error) {
	return nil, nil, errors.New("a pull request retry marks no task done")
}

func (service *fakeDrudgerService) EditTask(projectSlug string, id task.TaskID, changes task.EditTaskDto) (*task.Task, error) {
	return nil, errors.New("a pull request retry edits no task")
}

func (service *fakeDrudgerService) EditTaskFromDraft(projectSlug string, id task.TaskID) (*task.Task, []string, error) {
	return nil, nil, errors.New("a pull request retry edits no task")
}

func (service *fakeDrudgerService) RetryPullRequests(projectSlug string, requestedID task.TaskID) (drudger.PullRequestsRetried, error) {
	if projectSlug != testProjectSlug {
		return drudger.PullRequestsRetried{}, errors.New("the retry went to another project")
	}
	service.retryIDs = append(service.retryIDs, requestedID)
	return service.retried, service.retryErr
}

func (service *fakeDrudgerService) ReclaimDrudgers(projectSlug string) ([]drudger.FreedSlot, error) {
	return nil, errors.New("a pull request retry reclaims no slot")
}

func (service *fakeDrudgerService) NukeDrudger(projectSlug string, slot int, isForced bool) error {
	return errors.New("a pull request retry nukes no Drudger")
}

func (service *fakeDrudgerService) ReadDrudgers(projectSlug string) ([]*drudger.Drudger, error) {
	return nil, errors.New("a pull request retry reads no Drudgers")
}

func TestKind(t *testing.T) {
	cases := []struct {
		name          string
		service       *fakeDrudgerService
		wantVerdict   string
		wantIsSkipped bool
		wantErr       error
	}{
		{name: "retry counts the one pull request it opened", service: &fakeDrudgerService{retried: drudger.PullRequestsRetried{Tried: 1, Opened: 1}}, wantVerdict: "opened 1 pull request of 3f9a1c2e"},
		{name: "retry counts the pull requests it opened", service: &fakeDrudgerService{retried: drudger.PullRequestsRetried{Tried: 2, Opened: 2}}, wantVerdict: "opened 2 pull requests of 3f9a1c2e"},
		{name: "retry with none left skips", service: &fakeDrudgerService{}, wantVerdict: "no pull requests of 3f9a1c2e left to open", wantIsSkipped: true},
		{name: "retry with a failed pull request counts the ones it tried", service: &fakeDrudgerService{retried: drudger.PullRequestsRetried{Tried: 2, Opened: 1}}, wantVerdict: "opened 1 of 2 pull requests of 3f9a1c2e"},
		{name: "retry with every pull request failed skips", service: &fakeDrudgerService{retried: drudger.PullRequestsRetried{Tried: 2}}, wantVerdict: "opened 0 of 2 pull requests of 3f9a1c2e", wantIsSkipped: true},
		{name: "refused retry carries the error of the service", service: &fakeDrudgerService{retryErr: errStillWorking}, wantErr: errStillWorking},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			commands := actions.New(actions.Services{Drudgers: testCase.service}, testProjectSlug, []actions.Kind{pullrequests.Kind})
			_, cmd := commands.Start(views.PullRequestsMsg{TaskID: testTaskID})
			done, ok := cmd().(actions.DoneMsg)
			if !ok {
				t.Fatal("expected a done message")
			}
			if !slices.Equal(testCase.service.retryIDs, []task.TaskID{testTaskID}) {
				t.Errorf("expected one retry of %s, got %v", testTaskID, testCase.service.retryIDs)
			}
			wantCommand := actions.Command{Name: views.PullRequestsCommand, Target: "3f9a1c2e"}
			if done.Command != wantCommand {
				t.Errorf("expected done command %v, got %v", wantCommand, done.Command)
			}
			if done.Verdict != testCase.wantVerdict {
				t.Errorf("expected verdict %q, got %q", testCase.wantVerdict, done.Verdict)
			}
			if done.IsSkipped != testCase.wantIsSkipped {
				t.Errorf("expected skipped %v, got %v", testCase.wantIsSkipped, done.IsSkipped)
			}
			if !errors.Is(done.Err, testCase.wantErr) {
				t.Errorf("expected error %v, got %v", testCase.wantErr, done.Err)
			}
			wantFeeds := []views.FeedName{views.TasksFeed}
			if !slices.Equal(done.Feeds, wantFeeds) {
				t.Errorf("expected feeds %v, got %v", wantFeeds, done.Feeds)
			}
		})
	}
}
