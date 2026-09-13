package task

import (
	"errors"
	"strings"
	"testing"

	"drudge/internal/common"
)

// fakeSessionKeeper stands for the drudger service. The live Session check and
// the run directory both come from it.
type fakeSessionKeeper struct {
	fakeSessionGuard
	// hadRun is what the keeper reports about the run directory of the task.
	hadRun     bool
	runFailure error
	runRemoved TaskID
}

func (keeper *fakeSessionKeeper) RemoveRun(taskID TaskID) (bool, error) {
	keeper.runRemoved = taskID
	return keeper.hadRun, keeper.runFailure
}

// fakeConfirmation answers a removal the way a user would, and records the
// task it was asked about.
type fakeConfirmation struct {
	approved bool
	failure  error
	asked    TaskID
}

func (confirmation *fakeConfirmation) answer(taskToRemove *Task) (bool, error) {
	confirmation.asked = taskToRemove.ID
	return confirmation.approved, confirmation.failure
}

func TestTaskService_RemoveTask(t *testing.T) {
	cases := []struct {
		name  string
		force bool
		// approved is what the user answers when the removal asks.
		approved bool
		// hadRun says whether the task left a run directory behind.
		hadRun bool

		wantRemoved bool
		wantAsked   bool
	}{
		{
			name:        "a confirmed removal",
			approved:    true,
			wantRemoved: true,
			wantAsked:   true,
		},
		{
			name:        "a confirmed removal of a task that ran",
			approved:    true,
			hadRun:      true,
			wantRemoved: true,
			wantAsked:   true,
		},
		{
			name:      "a declined removal",
			wantAsked: true,
		},
		{
			name:        "a forced removal",
			force:       true,
			wantRemoved: true,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			repo := &fakeTaskRepo{tasks: []*Task{editableTask()}}
			service := NewTaskService(repo, common.NewLogger(""))
			keeper := &fakeSessionKeeper{hadRun: testCase.hadRun}
			confirmation := &fakeConfirmation{approved: testCase.approved}

			err := service.RemoveTask(testProjectSlug, editableTaskID, testCase.force, keeper, confirmation.answer)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			_, lookupErr := repo.GetTask(testProjectSlug, editableTaskID)
			removed := lookupErr != nil
			if removed != testCase.wantRemoved {
				t.Errorf("expected the task to be removed: %v, got %v", testCase.wantRemoved, removed)
			}

			asked := confirmation.asked != ""
			if asked != testCase.wantAsked {
				t.Errorf("expected the removal to ask the user: %v, got %v", testCase.wantAsked, asked)
			}

			wantRunRemoved := TaskID("")
			if testCase.wantRemoved {
				wantRunRemoved = editableTaskID
			}
			if keeper.runRemoved != wantRunRemoved {
				t.Errorf("expected the run directory of task %q to be removed, got %q", wantRunRemoved, keeper.runRemoved)
			}
		})
	}
}

func TestTaskService_RemoveTask_ResolvesAnIDPrefix(t *testing.T) {
	repo := &fakeTaskRepo{tasks: []*Task{editableTask()}}
	service := NewTaskService(repo, common.NewLogger(""))
	keeper := &fakeSessionKeeper{}

	if err := service.RemoveTask(testProjectSlug, "006684e3", true, keeper, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if keeper.runRemoved != editableTaskID {
		t.Errorf("expected the removal to resolve to task %q, got %q", editableTaskID, keeper.runRemoved)
	}
	if _, err := repo.GetTask(testProjectSlug, editableTaskID); err == nil {
		t.Error("expected the task to be gone")
	}
}

func TestTaskService_RemoveTask_RefusesATaskAnAgentIsWorkingOn(t *testing.T) {
	stored := editableTask()
	stored.Status = StatusInProgress
	repo := &fakeTaskRepo{tasks: []*Task{stored}}
	service := NewTaskService(repo, common.NewLogger(""))

	keeper := &fakeSessionKeeper{}
	keeper.refusal = errors.New("Drudger 1 (drudge-claude-demo-1) is still working on this task")
	confirmation := &fakeConfirmation{approved: true}

	err := service.RemoveTask(testProjectSlug, editableTaskID, false, keeper, confirmation.answer)
	if err == nil {
		t.Fatal("expected the removal of a task with a live Session to be refused")
	}
	if !strings.Contains(err.Error(), "drudge-claude-demo-1") {
		t.Errorf("expected the error to name the Drudger, got %q", err)
	}
	if _, lookupErr := repo.GetTask(testProjectSlug, editableTaskID); lookupErr != nil {
		t.Error("expected the task to be left alone")
	}
	if keeper.runRemoved != "" {
		t.Errorf("expected the run directory to be left alone, got %q", keeper.runRemoved)
	}
	if confirmation.asked != "" {
		t.Error("expected a refused removal to ask the user nothing")
	}
}

func TestTaskService_RemoveTask_TakesATaskWhoseSessionHasFinished(t *testing.T) {
	stored := editableTask()
	stored.Status = StatusFuckedUp
	repo := &fakeTaskRepo{tasks: []*Task{stored}}
	service := NewTaskService(repo, common.NewLogger(""))

	keeper := &fakeSessionKeeper{hadRun: true}
	if err := service.RemoveTask(testProjectSlug, editableTaskID, true, keeper, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := repo.GetTask(testProjectSlug, editableTaskID); err == nil {
		t.Error("expected the task to be gone")
	}
}

func TestTaskService_RemoveTask_RefusesAnEmptyID(t *testing.T) {
	repo := &fakeTaskRepo{tasks: []*Task{editableTask()}}
	service := NewTaskService(repo, common.NewLogger(""))

	err := service.RemoveTask(testProjectSlug, "", true, &fakeSessionKeeper{}, nil)
	if !errors.Is(err, ErrNoTaskID) {
		t.Fatalf("expected %v, got %v", ErrNoTaskID, err)
	}
}

func TestTaskService_RemoveTask_SurfacesTheLookupFailure(t *testing.T) {
	cases := []struct {
		name     string
		id       TaskID
		wantText string
	}{
		{name: "an id nobody carries", id: "ffffffff", wantText: "ffffffff"},
		{name: "a prefix several tasks carry", id: "00", wantText: "Fix logout"},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			other := editableTask()
			other.ID = "0071a2b4-dbe9-4316-8aba-8a67a8f01f8f"
			other.Title = "Fix logout"

			repo := &fakeTaskRepo{tasks: []*Task{editableTask(), other}}
			service := NewTaskService(repo, common.NewLogger(""))

			err := service.RemoveTask(testProjectSlug, testCase.id, true, &fakeSessionKeeper{}, nil)
			if err == nil {
				t.Fatal("expected the lookup failure to surface")
			}
			if !strings.Contains(err.Error(), testCase.wantText) {
				t.Errorf("expected the error to mention %q, got %q", testCase.wantText, err)
			}
			if len(repo.tasks) != 2 {
				t.Errorf("expected both tasks to stay, got %d", len(repo.tasks))
			}
		})
	}
}

func TestTaskService_RemoveTask_ReportsAnotherCommandHoldingTheTask(t *testing.T) {
	repo := &fakeTaskRepo{
		tasks:  []*Task{editableTask()},
		locked: map[TaskID]bool{editableTaskID: true},
	}
	service := NewTaskService(repo, common.NewLogger(""))

	err := service.RemoveTask(testProjectSlug, editableTaskID, true, &fakeSessionKeeper{}, nil)
	if err == nil {
		t.Fatal("expected the removal of a locked task to be refused")
	}
	if !strings.Contains(err.Error(), string(editableTaskID)) {
		t.Errorf("expected the error to name the task, got %q", err)
	}
}

func TestTaskService_RemoveTask_ReportsARunDirectoryLeftBehind(t *testing.T) {
	repo := &fakeTaskRepo{tasks: []*Task{editableTask()}}
	service := NewTaskService(repo, common.NewLogger(""))
	keeper := &fakeSessionKeeper{runFailure: errors.New("permission denied")}

	err := service.RemoveTask(testProjectSlug, editableTaskID, true, keeper, nil)
	if err == nil {
		t.Fatal("expected a run directory that could not be removed to surface")
	}
	if !strings.Contains(err.Error(), string(editableTaskID)) {
		t.Errorf("expected the error to name the task, got %q", err)
	}
}
