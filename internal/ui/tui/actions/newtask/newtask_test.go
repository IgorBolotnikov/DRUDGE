package newtask_test

import (
	"errors"
	"reflect"
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

// fakeTaskService refuses every task with createErr and records the fields of
// every task it is asked to add.
type fakeTaskService struct {
	createErr error
	dtos      []task.CreateTaskDto
}

func (service *fakeTaskService) CreateTask(dto task.CreateTaskDto) (*task.Task, error) {
	service.dtos = append(service.dtos, dto)
	if service.createErr != nil {
		return nil, service.createErr
	}
	return &task.Task{ID: testTaskID, Title: dto.Title}, nil
}

func TestKind(t *testing.T) {
	draft := task.Draft{Title: "Add retry", Status: task.StatusDraft, BlockedBy: []task.TaskID{"a01f3b55"}, Description: "Retry the upload."}
	cases := []struct {
		name        string
		service     *fakeTaskService
		wantVerdict string
		wantErr     error
	}{
		{name: "create names the new task", service: &fakeTaskService{}, wantVerdict: "3f9a1c2e  Add retry added"},
		{name: "refused create carries the error of the service", service: &fakeTaskService{createErr: errUnknownBlocker}, wantErr: errUnknownBlocker},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			commands := actions.New(actions.Services{Tasks: testCase.service}, testProjectSlug, []actions.Kind{newtask.Kind})
			_, cmd := commands.Start(views.NewTaskMsg{Draft: draft})
			done, ok := cmd().(actions.DoneMsg)
			if !ok {
				t.Fatal("expected a done message")
			}
			wantDtos := []task.CreateTaskDto{draft.Dto(testProjectSlug)}
			if !reflect.DeepEqual(testCase.service.dtos, wantDtos) {
				t.Errorf("expected one create of %+v, got %+v", wantDtos, testCase.service.dtos)
			}
			wantCommand := actions.Command{Name: views.NewTaskCommand, Target: "Add retry"}
			if done.Command != wantCommand {
				t.Errorf("expected done command %v, got %v", wantCommand, done.Command)
			}
			if done.Verdict != testCase.wantVerdict {
				t.Errorf("expected verdict %q, got %q", testCase.wantVerdict, done.Verdict)
			}
			if !errors.Is(done.Err, testCase.wantErr) {
				t.Errorf("expected error %v, got %v", testCase.wantErr, done.Err)
			}
			if wantFeeds := []views.FeedName{views.TasksFeed}; !slices.Equal(done.Feeds, wantFeeds) {
				t.Errorf("expected feeds %v, got %v", wantFeeds, done.Feeds)
			}
		})
	}
}
