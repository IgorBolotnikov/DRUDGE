package removetask_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/IgorBolotnikov/DRUDGE/internal/task"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/actions"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/actions/removetask"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/views"
)

const testProjectSlug = "uploader"

const testTaskID task.TaskID = "3f9a1c2e-0000-4000-8000-000000000001"

var errStillWorking = errors.New("task 3f9a1c2e has an agent still working on it")

// fakeTaskService removes a task once the confirmation approves the removal of
// removal. It refuses with removeErr and records the task of every removal.
type fakeTaskService struct {
	removal   task.Removal
	removeErr error
	removedID []task.TaskID
}

func (service *fakeTaskService) OpenDraft(projectSlug string) (string, error) {
	return "", errors.New("a removal opens no draft")
}

func (service *fakeTaskService) OpenTaskDraft(projectSlug string, id task.TaskID) (string, error) {
	return "", errors.New("a removal opens no draft")
}

func (service *fakeTaskService) CreateTaskFromDraft(projectSlug string) (*task.Task, error) {
	return nil, errors.New("a removal adds no task")
}

func (service *fakeTaskService) RemoveTask(projectSlug string, id task.TaskID, isForced bool, sessions task.SessionKeeper, confirm task.ConfirmRemoval) error {
	if projectSlug != testProjectSlug || isForced {
		return errors.New("the removal went to another project or was forced")
	}
	if service.removeErr != nil {
		return service.removeErr
	}
	isApproved, err := confirm(service.removal)
	if err != nil || !isApproved {
		return err
	}
	service.removedID = append(service.removedID, id)
	return nil
}

// fakeAsker answers every question with isYes and records the questions.
type fakeAsker struct {
	isYes     bool
	questions []string
}

func (asker *fakeAsker) Confirm(question string) bool {
	asker.questions = append(asker.questions, question)
	return asker.isYes
}

func TestKind(t *testing.T) {
	removal := task.Removal{
		Task:       &task.Task{ID: testTaskID, Title: "Add retry"},
		Dependents: []*task.Task{{ID: "4d5e6f70-0000-4000-8000-000000000002", Title: "Ship it"}},
	}
	wantQuestion := "task " + string(testTaskID) + " \"Add retry\"\n1 task is blocked by it and will be unblocked:\n  4d5e6f70  Ship it"
	cases := []struct {
		name          string
		isYes         bool
		removeErr     error
		wantQuestions []string
		wantRemoved   []task.TaskID
		wantVerdict   string
		wantIsSkipped bool
		wantErr       error
	}{
		{name: "yes removes the task", isYes: true, wantQuestions: []string{wantQuestion}, wantRemoved: []task.TaskID{testTaskID}, wantVerdict: "3f9a1c2e removed"},
		{name: "no keeps the task", wantQuestions: []string{wantQuestion}, wantVerdict: "3f9a1c2e kept", wantIsSkipped: true},
		{name: "refused removal carries the error of the service", isYes: true, removeErr: errStillWorking, wantErr: errStillWorking},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			service := &fakeTaskService{removal: removal, removeErr: testCase.removeErr}
			asker := &fakeAsker{isYes: testCase.isYes}
			commands := actions.New(actions.Services{Tasks: service, Asker: asker}, testProjectSlug, []actions.Kind{removetask.Kind})

			command, cmd := commands.Start(views.RemoveTaskMsg{TaskID: testTaskID})
			done, ok := cmd().(actions.DoneMsg)

			if !ok {
				t.Fatal("expected a done message")
			}
			if !slices.Equal(asker.questions, testCase.wantQuestions) {
				t.Errorf("expected questions %q, got %q", testCase.wantQuestions, asker.questions)
			}
			if !slices.Equal(service.removedID, testCase.wantRemoved) {
				t.Errorf("expected removals %v, got %v", testCase.wantRemoved, service.removedID)
			}
			wantCommand := actions.Command{Name: views.RemoveTaskCommand, Target: "3f9a1c2e"}
			if command != wantCommand || done.Command != wantCommand {
				t.Errorf("expected command %v, got %v and %v", wantCommand, command, done.Command)
			}
			if done.Verdict != testCase.wantVerdict || done.IsSkipped != testCase.wantIsSkipped {
				t.Errorf("expected verdict %q (skipped %v), got %q (skipped %v)", testCase.wantVerdict, testCase.wantIsSkipped, done.Verdict, done.IsSkipped)
			}
			if !errors.Is(done.Err, testCase.wantErr) {
				t.Errorf("expected error %v, got %v", testCase.wantErr, done.Err)
			}
			if !slices.Equal(done.Feeds, []views.FeedName{views.TasksFeed}) {
				t.Errorf("expected the tasks feed, got %v", done.Feeds)
			}
		})
	}
}
