package setstatus_test

import (
	"errors"
	"testing"

	"github.com/IgorBolotnikov/DRUDGE/internal/drudger"
	"github.com/IgorBolotnikov/DRUDGE/internal/task"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/actions"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/actions/setstatus"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/views"
)

const testProjectSlug = "uploader"

const testTaskID task.TaskID = "3f9a1c2e-0000-4000-8000-000000000001"

var errStillWorking = errors.New("task 3f9a1c2e has an agent still working on it")

// fakeDrudgerService edits a task to the status of the edit unless it refuses
// with editErr. It records the edits.
type fakeDrudgerService struct {
	editErr error
	edits   []task.EditTaskDto
}

func (service *fakeDrudgerService) RunTask(projectSlug string, requestedID task.TaskID, isDryRun bool) error {
	return errors.New("a status change runs no task")
}

func (service *fakeDrudgerService) RerunTask(projectSlug string, requestedID task.TaskID, isDryRun bool) error {
	return errors.New("a status change reruns no task")
}

func (service *fakeDrudgerService) MarkDone(projectSlug string, id task.TaskID) (*task.Task, []*task.Task, error) {
	return nil, nil, errors.New("a status change marks no task done")
}

func (service *fakeDrudgerService) EditTask(projectSlug string, id task.TaskID, changes task.EditTaskDto) (*task.Task, error) {
	if projectSlug != testProjectSlug || id != testTaskID {
		return nil, errors.New("the edit went to another project or task")
	}
	service.edits = append(service.edits, changes)
	if service.editErr != nil {
		return nil, service.editErr
	}
	return &task.Task{ID: id, Status: *changes.Status}, nil
}

func (service *fakeDrudgerService) EditTaskFromDraft(projectSlug string, id task.TaskID) (*task.Task, []string, error) {
	return nil, nil, errors.New("a status change edits no draft")
}

func (service *fakeDrudgerService) RetryPullRequests(projectSlug string, requestedID task.TaskID) (drudger.PullRequestsRetried, error) {
	return drudger.PullRequestsRetried{}, errors.New("a status change opens no pull request")
}

func (service *fakeDrudgerService) ReclaimDrudgers(projectSlug string) ([]drudger.FreedSlot, error) {
	return nil, errors.New("a status change reclaims no slot")
}

func (service *fakeDrudgerService) NukeDrudger(projectSlug string, slot int, isForced bool) error {
	return errors.New("a status change nukes no Drudger")
}

func (service *fakeDrudgerService) ReadDrudgers(projectSlug string) ([]*drudger.Drudger, error) {
	return nil, errors.New("a status change reads no Drudgers")
}

func TestKind(t *testing.T) {
	cases := []struct {
		name        string
		msg         views.SetStatusMsg
		editErr     error
		wantForced  bool
		wantVerdict string
		wantErr     error
	}{
		{name: "a status change names the task and its status", msg: views.SetStatusMsg{TaskID: testTaskID, Status: task.StatusTodo}, wantVerdict: "3f9a1c2e now todo"},
		{name: "done goes through the same edit", msg: views.SetStatusMsg{TaskID: testTaskID, Status: task.StatusDone}, wantVerdict: "3f9a1c2e now done"},
		{name: "a forced change sends force", msg: views.SetStatusMsg{TaskID: testTaskID, Status: task.StatusInProgress, IsForced: true}, wantForced: true, wantVerdict: "3f9a1c2e now in-progress"},
		{name: "a refusal carries the error of the service", msg: views.SetStatusMsg{TaskID: testTaskID, Status: task.StatusTodo}, editErr: errStillWorking, wantErr: errStillWorking},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			service := &fakeDrudgerService{editErr: testCase.editErr}
			commands := actions.New(actions.Services{Drudgers: service}, testProjectSlug, []actions.Kind{setstatus.Kind})
			command, cmd := commands.Start(testCase.msg)
			done, ok := cmd().(actions.DoneMsg)
			if !ok {
				t.Fatal("expected a done message")
			}
			wantCommand := actions.Command{Name: views.SetStatusCommand, Target: "3f9a1c2e"}
			if command != wantCommand || done.Command != wantCommand {
				t.Errorf("expected command %v, got %v and %v", wantCommand, command, done.Command)
			}
			if len(service.edits) != 1 {
				t.Fatalf("expected one edit, got %d", len(service.edits))
			}
			edit := service.edits[0]
			if edit.Status == nil || *edit.Status != testCase.msg.Status {
				t.Errorf("expected the edit to set status %q, got %v", testCase.msg.Status, edit.Status)
			}
			if edit.AllowsManagedStatus != testCase.wantForced {
				t.Errorf("expected force %v, got %v", testCase.wantForced, edit.AllowsManagedStatus)
			}
			editedOnlyStatus := edit
			editedOnlyStatus.Status = nil
			if editedOnlyStatus.HasChanges() {
				t.Errorf("expected the edit to change only the status, got %+v", edit)
			}
			if !errors.Is(done.Err, testCase.wantErr) {
				t.Errorf("expected error %v, got %v", testCase.wantErr, done.Err)
			}
			if done.Verdict != testCase.wantVerdict {
				t.Errorf("expected verdict %q, got %q", testCase.wantVerdict, done.Verdict)
			}
			if len(done.Feeds) != 1 || done.Feeds[0] != views.TasksFeed {
				t.Errorf("expected the tasks feed to refresh, got %v", done.Feeds)
			}
		})
	}
}

func TestKind_Question(t *testing.T) {
	commands := actions.New(actions.Services{Drudgers: &fakeDrudgerService{}}, testProjectSlug, []actions.Kind{setstatus.Kind})
	cases := []struct {
		name         string
		msg          views.SetStatusMsg
		wantQuestion string
		wantOK       bool
	}{
		{name: "a forced status asks", msg: views.SetStatusMsg{TaskID: testTaskID, Status: task.StatusInProgress, IsForced: true}, wantQuestion: "Force 3f9a1c2e to in-progress? drudge maintains this status itself.", wantOK: true},
		{name: "any other status starts right away", msg: views.SetStatusMsg{TaskID: testTaskID, Status: task.StatusTodo}},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			question, ok := commands.Question(testCase.msg)
			if question != testCase.wantQuestion || ok != testCase.wantOK {
				t.Errorf("expected question %q (ok %v), got %q (ok %v)", testCase.wantQuestion, testCase.wantOK, question, ok)
			}
		})
	}
}
