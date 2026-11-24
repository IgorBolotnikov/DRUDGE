package reclaim_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/IgorBolotnikov/DRUDGE/internal/drudger"
	"github.com/IgorBolotnikov/DRUDGE/internal/task"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/actions"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/actions/reclaim"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/views"
)

const testProjectSlug = "uploader"

var errDiskOnFire = errors.New("disk on fire")

// fakeDrudgerService frees the freed slots on every reclaim or fails it with
// reclaimErr. It counts the reclaims.
type fakeDrudgerService struct {
	freed        []drudger.FreedSlot
	reclaimErr   error
	reclaimCount int
}

func (service *fakeDrudgerService) RunTask(projectSlug string, requestedID task.TaskID, isDryRun bool) error {
	return errors.New("a reclaim runs no task")
}

func (service *fakeDrudgerService) RerunTask(projectSlug string, requestedID task.TaskID, isDryRun bool) error {
	return errors.New("a reclaim reruns no task")
}

func (service *fakeDrudgerService) MarkDone(projectSlug string, id task.TaskID) (*task.Task, []*task.Task, error) {
	return nil, nil, errors.New("a reclaim marks no task done")
}

func (service *fakeDrudgerService) EditTask(projectSlug string, id task.TaskID, changes task.EditTaskDto) (*task.Task, error) {
	return nil, errors.New("this test edits no task")
}

func (service *fakeDrudgerService) EditTaskFromDraft(projectSlug string, id task.TaskID) (*task.Task, []string, error) {
	return nil, nil, errors.New("a reclaim edits no task")
}

func (service *fakeDrudgerService) RetryPullRequests(projectSlug string, requestedID task.TaskID) (drudger.PullRequestsRetried, error) {
	return drudger.PullRequestsRetried{}, errors.New("a reclaim opens no pull request")
}

func (service *fakeDrudgerService) ReclaimDrudgers(projectSlug string) ([]drudger.FreedSlot, error) {
	if projectSlug != testProjectSlug {
		return nil, errors.New("the reclaim went to another project")
	}
	service.reclaimCount++
	return service.freed, service.reclaimErr
}

func (service *fakeDrudgerService) NukeDrudger(projectSlug string, slot int, isForced bool) error {
	return errors.New("a reclaim nukes no Drudger")
}

func (service *fakeDrudgerService) ReadDrudgers(projectSlug string) ([]*drudger.Drudger, error) {
	return nil, errors.New("a reclaim reads no Drudgers")
}

func TestKind(t *testing.T) {
	oneFreed := []drudger.FreedSlot{{Slot: 2, TaskID: "1a2b3c4d-0000-4000-8000-000000000001"}}
	twoFreed := []drudger.FreedSlot{
		{Slot: 2, TaskID: "1a2b3c4d-0000-4000-8000-000000000001"},
		{Slot: 4, TaskID: "4d5e6f70-0000-4000-8000-000000000002"},
	}
	reclaimFeeds := []views.FeedName{views.DrudgersFeed}
	cases := []struct {
		name          string
		service       *fakeDrudgerService
		wantVerdict   string
		wantIsSkipped bool
		wantErr       error
	}{
		{name: "one slot freed", service: &fakeDrudgerService{freed: oneFreed}, wantVerdict: "freed slot 2 · rerun 1a2b3c4d to start it over"},
		{name: "two slots freed", service: &fakeDrudgerService{freed: twoFreed}, wantVerdict: "freed slots 2, 4 · rerun 1a2b3c4d, 4d5e6f70 to start them over"},
		{name: "nothing freed", service: &fakeDrudgerService{}, wantVerdict: "no slot was claimed by a gone agent", wantIsSkipped: true},
		{name: "failed reclaim carries the error of the service", service: &fakeDrudgerService{reclaimErr: errDiskOnFire}, wantErr: errDiskOnFire},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			commands := actions.New(actions.Services{Drudgers: testCase.service}, testProjectSlug, []actions.Kind{reclaim.Kind})
			_, cmd := commands.Start(views.ReclaimMsg{})
			done, ok := cmd().(actions.DoneMsg)
			if !ok {
				t.Fatal("expected a done message")
			}
			if testCase.service.reclaimCount != 1 {
				t.Errorf("expected one reclaim, got %d", testCase.service.reclaimCount)
			}
			wantCommand := actions.Command{Name: views.ReclaimCommand, Target: "slots"}
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
			if !slices.Equal(done.Feeds, reclaimFeeds) {
				t.Errorf("expected feeds %v, got %v", reclaimFeeds, done.Feeds)
			}
		})
	}
}
