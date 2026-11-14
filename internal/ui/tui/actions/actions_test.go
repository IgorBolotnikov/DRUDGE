package actions_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/IgorBolotnikov/DRUDGE/internal/drudger"
	"github.com/IgorBolotnikov/DRUDGE/internal/task"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/actions"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/views"
)

const testProjectSlug = "uploader"

const testTaskID task.TaskID = "3f9a1c2e-0000-4000-8000-000000000001"

var errNotTodo = errors.New(`task 3f9a1c2e is "in progress", only a task that is "todo" can run`)

var errNotUnmerged = errors.New(`task 3f9a1c2e is "todo", only a task that is "unmerged" can be marked done`)

var errDiskOnFire = errors.New("disk on fire")

// fakeDrudgerService refuses every run with runErr, every mark done with
// doneErr and fails every read with readErr. A mark done unblocks the
// unblocked tasks. It records the task of every run and every mark done.
type fakeDrudgerService struct {
	drudgers  []*drudger.Drudger
	unblocked []*task.Task
	runErr    error
	doneErr   error
	readErr   error
	runIDs    []task.TaskID
	doneIDs   []task.TaskID
}

func (service *fakeDrudgerService) RunTask(projectSlug string, requestedID task.TaskID, isDryRun bool) error {
	if projectSlug != testProjectSlug || isDryRun {
		return errors.New("the run went to another project or was a dry run")
	}
	service.runIDs = append(service.runIDs, requestedID)
	return service.runErr
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

func (service *fakeDrudgerService) ReadDrudgers(projectSlug string) ([]*drudger.Drudger, error) {
	return service.drudgers, service.readErr
}

func TestActions_RunTask(t *testing.T) {
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
			commands := actions.New(actions.Services{Drudgers: testCase.service}, testProjectSlug)
			command, cmd := commands.Start(views.RunTaskMsg{TaskID: testTaskID})
			wantCommand := actions.Command{Name: actions.RunCommand, Target: "3f9a1c2e"}
			if command != wantCommand {
				t.Errorf("expected command %v, got %v", wantCommand, command)
			}
			if len(testCase.service.runIDs) != 0 {
				t.Fatal("expected the run to wait for its command")
			}
			done, ok := cmd().(actions.DoneMsg)
			if !ok {
				t.Fatal("expected a done message")
			}
			if !slices.Equal(testCase.service.runIDs, []task.TaskID{testTaskID}) {
				t.Errorf("expected one run of %s, got %v", testTaskID, testCase.service.runIDs)
			}
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

func TestActions_MarkDone(t *testing.T) {
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
			commands := actions.New(actions.Services{Drudgers: testCase.service}, testProjectSlug)
			command, cmd := commands.Start(views.MarkDoneMsg{TaskID: testTaskID})
			wantCommand := actions.Command{Name: actions.MarkDoneCommand, Target: "3f9a1c2e"}
			if command != wantCommand {
				t.Errorf("expected command %v, got %v", wantCommand, command)
			}
			if len(testCase.service.doneIDs) != 0 {
				t.Fatal("expected the mark done to wait for its command")
			}
			done, ok := cmd().(actions.DoneMsg)
			if !ok {
				t.Fatal("expected a done message")
			}
			if !slices.Equal(testCase.service.doneIDs, []task.TaskID{testTaskID}) {
				t.Errorf("expected one mark done of %s, got %v", testTaskID, testCase.service.doneIDs)
			}
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

func TestCommand_String(t *testing.T) {
	command := actions.Command{Name: actions.RunCommand, Target: "3f9a1c2e"}
	if got := command.String(); got != "run 3f9a1c2e" {
		t.Errorf("expected run 3f9a1c2e, got %q", got)
	}
}
