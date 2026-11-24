package runtask_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/IgorBolotnikov/DRUDGE/internal/drudger"
	"github.com/IgorBolotnikov/DRUDGE/internal/task"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/actions"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/actions/runtask"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/views"
)

const testProjectSlug = "uploader"

const testTaskID task.TaskID = "3f9a1c2e-0000-4000-8000-000000000001"

var errNotTodo = errors.New(`task 3f9a1c2e is "in progress", only a task that is "todo" can run`)

var errDiskOnFire = errors.New("disk on fire")

// fakeDrudgerService refuses every run with runErr and fails every read with
// readErr. It records the task of every run.
type fakeDrudgerService struct {
	drudgers []*drudger.Drudger
	runErr   error
	readErr  error
	runIDs   []task.TaskID
}

func (service *fakeDrudgerService) RunTask(projectSlug string, requestedID task.TaskID, isDryRun bool) error {
	if projectSlug != testProjectSlug || isDryRun {
		return errors.New("the run went to another project or was a dry run")
	}
	service.runIDs = append(service.runIDs, requestedID)
	return service.runErr
}

func (service *fakeDrudgerService) RerunTask(projectSlug string, requestedID task.TaskID, isDryRun bool) error {
	return errors.New("a run reruns no task")
}

func (service *fakeDrudgerService) MarkDone(projectSlug string, id task.TaskID) (*task.Task, []*task.Task, error) {
	return nil, nil, errors.New("a run marks no task done")
}

func (service *fakeDrudgerService) EditTaskFromDraft(projectSlug string, id task.TaskID) (*task.Task, []string, error) {
	return nil, nil, errors.New("a run edits no task")
}

func (service *fakeDrudgerService) ReclaimDrudgers(projectSlug string) ([]drudger.FreedSlot, error) {
	return nil, errors.New("a run reclaims no slot")
}

func (service *fakeDrudgerService) ReadDrudgers(projectSlug string) ([]*drudger.Drudger, error) {
	return service.drudgers, service.readErr
}

func TestKind(t *testing.T) {
	holders := []*drudger.Drudger{{Slot: 1, TaskID: "another"}, {Slot: 2, TaskID: testTaskID}}
	runFeeds := []views.FeedName{views.TasksFeed, views.DrudgersFeed}
	cases := []struct {
		name        string
		service     *fakeDrudgerService
		wantVerdict string
		wantErr     error
	}{
		{name: "run names the Drudger that took the task", service: &fakeDrudgerService{drudgers: holders}, wantVerdict: "3f9a1c2e handed to Drudger 2"},
		{name: "run with no Drudger holding the task", service: &fakeDrudgerService{}, wantVerdict: "3f9a1c2e handed to a Drudger"},
		{name: "run whose Drudgers cannot be read", service: &fakeDrudgerService{drudgers: holders, readErr: errDiskOnFire}, wantVerdict: "3f9a1c2e handed to a Drudger"},
		{name: "refused run carries the error of the service", service: &fakeDrudgerService{runErr: errNotTodo}, wantErr: errNotTodo},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			commands := actions.New(actions.Services{Drudgers: testCase.service}, testProjectSlug, []actions.Kind{runtask.Kind})
			_, cmd := commands.Start(views.RunTaskMsg{TaskID: testTaskID})
			done, ok := cmd().(actions.DoneMsg)
			if !ok {
				t.Fatal("expected a done message")
			}
			if !slices.Equal(testCase.service.runIDs, []task.TaskID{testTaskID}) {
				t.Errorf("expected one run of %s, got %v", testTaskID, testCase.service.runIDs)
			}
			wantCommand := actions.Command{Name: views.RunTaskCommand, Target: "3f9a1c2e"}
			if done.Command != wantCommand {
				t.Errorf("expected done command %v, got %v", wantCommand, done.Command)
			}
			if done.Verdict != testCase.wantVerdict {
				t.Errorf("expected verdict %q, got %q", testCase.wantVerdict, done.Verdict)
			}
			if !errors.Is(done.Err, testCase.wantErr) {
				t.Errorf("expected error %v, got %v", testCase.wantErr, done.Err)
			}
			if !slices.Equal(done.Feeds, runFeeds) {
				t.Errorf("expected feeds %v, got %v", runFeeds, done.Feeds)
			}
		})
	}
}
