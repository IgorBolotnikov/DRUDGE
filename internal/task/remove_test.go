package task

import (
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/IgorBolotnikov/DRUDGE/internal/common"
)

// fakeSessionKeeper stands for the drudger service. The live Session check and
// the run directory both come from it.
type fakeSessionKeeper struct {
	fakeSessionGuard
	// hasRun is what the keeper reports about the run directory of the task.
	hasRun     bool
	runFailure error
	runRemoved TaskID
	// branchFailure is what the keeper answers when it is asked to clean up
	// the branches of the task.
	branchFailure   error
	branchesCleaned TaskID
}

func (keeper *fakeSessionKeeper) RemoveRun(taskID TaskID) (bool, error) {
	keeper.runRemoved = taskID
	return keeper.hasRun, keeper.runFailure
}

func (keeper *fakeSessionKeeper) RemoveEmptyBranches(removed *Task) error {
	keeper.branchesCleaned = removed.ID
	return keeper.branchFailure
}

// fakeConfirmation answers a removal the way a user would, and records the
// removal it was asked about.
type fakeConfirmation struct {
	isApproved bool
	failure    error
	asked      TaskID
	removal    Removal
}

func (confirmation *fakeConfirmation) answer(removal Removal) (bool, error) {
	confirmation.asked = removal.Task.ID
	confirmation.removal = removal
	return confirmation.isApproved, confirmation.failure
}

func TestTaskService_RemoveTask(t *testing.T) {
	cases := []struct {
		name     string
		isForced bool
		// approved is what the user answers when the removal asks.
		isApproved bool
		// hasRun says whether the task left a run directory behind.
		hasRun bool

		wantRemoved bool
		wantAsked   bool
	}{
		{
			name:        "a confirmed removal",
			isApproved:  true,
			wantRemoved: true,
			wantAsked:   true,
		},
		{
			name:        "a confirmed removal of a task that ran",
			isApproved:  true,
			hasRun:      true,
			wantRemoved: true,
			wantAsked:   true,
		},
		{
			name:      "a declined removal",
			wantAsked: true,
		},
		{
			name:        "a forced removal",
			isForced:    true,
			wantRemoved: true,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			repo := &fakeTaskRepo{tasks: []*Task{editableTask()}}
			service := NewTaskService(repo, common.NewLogger(""))
			keeper := &fakeSessionKeeper{hasRun: testCase.hasRun}
			confirmation := &fakeConfirmation{isApproved: testCase.isApproved}

			err := service.RemoveTask(testProjectSlug, editableTaskID, testCase.isForced, keeper, confirmation.answer)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			_, lookupErr := repo.GetTask(testProjectSlug, editableTaskID)
			isRemoved := lookupErr != nil
			if isRemoved != testCase.wantRemoved {
				t.Errorf("expected the task to be removed: %v, got %v", testCase.wantRemoved, isRemoved)
			}

			wasAsked := confirmation.asked != ""
			if wasAsked != testCase.wantAsked {
				t.Errorf("expected the removal to ask the user: %v, got %v", testCase.wantAsked, wasAsked)
			}

			wantRunRemoved := TaskID("")
			if testCase.wantRemoved {
				wantRunRemoved = editableTaskID
			}
			if keeper.runRemoved != wantRunRemoved {
				t.Errorf("expected the run directory of task %q to be removed, got %q", wantRunRemoved, keeper.runRemoved)
			}
			if keeper.branchesCleaned != wantRunRemoved {
				t.Errorf("expected the branches of task %q to be cleaned up, got %q", wantRunRemoved, keeper.branchesCleaned)
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
	confirmation := &fakeConfirmation{isApproved: true}

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

	keeper := &fakeSessionKeeper{hasRun: true}
	if err := service.RemoveTask(testProjectSlug, editableTaskID, true, keeper, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := repo.GetTask(testProjectSlug, editableTaskID); err == nil {
		t.Error("expected the task to be gone")
	}
}

func TestTaskService_RemoveTask_RemovesATaskWhoseBranchesStayBehind(t *testing.T) {
	repo := &fakeTaskRepo{tasks: []*Task{editableTask()}}
	service := NewTaskService(repo, common.NewLogger(""))
	keeper := &fakeSessionKeeper{branchFailure: errors.New("branch drudge/006684e3 is used by worktree at slot-1")}

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

func TestTaskService_RemoveTask_StripsItsLinks(t *testing.T) {
	const (
		removedID     TaskID = "9c8d7e6f-dbe9-4316-8aba-8a67a8f01f8f"
		firstID       TaskID = "4f2a1b3c-dbe9-4316-8aba-8a67a8f01f8f"
		secondID      TaskID = "7e6d5c4b-dbe9-4316-8aba-8a67a8f01f8f"
		otherBlocker  TaskID = "2b3c4d5e-dbe9-4316-8aba-8a67a8f01f8f"
		otherParentID TaskID = "1a2b3c4d-dbe9-4316-8aba-8a67a8f01f8f"
	)
	createdAt := time.Date(2026, time.September, 1, 12, 0, 0, 0, time.UTC)
	// linkedTask is a task made the given number of minutes after the removed
	// one.
	linkedTask := func(id TaskID, minutesLater int, blockedBy []TaskID, parentID TaskID) *Task {
		return &Task{
			ID:           id,
			Title:        "Task " + ShortID(id),
			Status:       StatusTodo,
			BlockedBy:    blockedBy,
			ParentTaskID: parentID,
			CreatedAt:    createdAt.Add(time.Duration(minutesLater) * time.Minute),
		}
	}
	// wantLinks is what one task should name once the removal is done.
	type wantLinks struct {
		blockedBy []TaskID
		parentID  TaskID
	}

	cases := []struct {
		name       string
		others     []*Task
		isForced   bool
		isApproved bool

		wantDependents []TaskID
		wantChildren   []TaskID
		wantRemoved    bool
		wantLinks      map[TaskID]wantLinks
	}{
		{
			name:        "a task with no links",
			others:      []*Task{linkedTask(firstID, 1, []TaskID{otherBlocker}, otherParentID)},
			isApproved:  true,
			wantRemoved: true,
			wantLinks:   map[TaskID]wantLinks{firstID: {blockedBy: []TaskID{otherBlocker}, parentID: otherParentID}},
		},
		{
			name: "dependents",
			others: []*Task{
				linkedTask(secondID, 2, []TaskID{otherBlocker, removedID}, ""),
				linkedTask(firstID, 1, []TaskID{removedID}, ""),
			},
			isApproved:     true,
			wantDependents: []TaskID{firstID, secondID},
			wantRemoved:    true,
			wantLinks: map[TaskID]wantLinks{
				firstID:  {},
				secondID: {blockedBy: []TaskID{otherBlocker}},
			},
		},
		{
			name: "children",
			others: []*Task{
				linkedTask(secondID, 2, nil, removedID),
				linkedTask(firstID, 1, nil, removedID),
			},
			isApproved:   true,
			wantChildren: []TaskID{firstID, secondID},
			wantRemoved:  true,
			wantLinks: map[TaskID]wantLinks{
				firstID:  {},
				secondID: {},
			},
		},
		{
			name: "a dependent and a child",
			others: []*Task{
				linkedTask(firstID, 1, []TaskID{removedID}, otherParentID),
				linkedTask(secondID, 2, []TaskID{otherBlocker}, removedID),
			},
			isApproved:     true,
			wantDependents: []TaskID{firstID},
			wantChildren:   []TaskID{secondID},
			wantRemoved:    true,
			wantLinks: map[TaskID]wantLinks{
				firstID:  {parentID: otherParentID},
				secondID: {blockedBy: []TaskID{otherBlocker}},
			},
		},
		{
			name:           "a task both blocked by it and belonging to it",
			others:         []*Task{linkedTask(firstID, 1, []TaskID{removedID}, removedID)},
			isApproved:     true,
			wantDependents: []TaskID{firstID},
			wantChildren:   []TaskID{firstID},
			wantRemoved:    true,
			wantLinks:      map[TaskID]wantLinks{firstID: {}},
		},
		{
			name: "a declined removal",
			others: []*Task{
				linkedTask(firstID, 1, []TaskID{removedID}, ""),
				linkedTask(secondID, 2, nil, removedID),
			},
			wantDependents: []TaskID{firstID},
			wantChildren:   []TaskID{secondID},
			wantLinks: map[TaskID]wantLinks{
				firstID:  {blockedBy: []TaskID{removedID}},
				secondID: {parentID: removedID},
			},
		},
		{
			name: "a forced removal",
			others: []*Task{
				linkedTask(firstID, 1, []TaskID{removedID}, ""),
				linkedTask(secondID, 2, nil, removedID),
			},
			isForced:    true,
			wantRemoved: true,
			wantLinks: map[TaskID]wantLinks{
				firstID:  {},
				secondID: {},
			},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			removed := linkedTask(removedID, 0, nil, "")
			repo := &fakeTaskRepo{tasks: append([]*Task{removed}, testCase.others...)}
			service := NewTaskService(repo, common.NewLogger(""))
			confirmation := &fakeConfirmation{isApproved: testCase.isApproved}

			err := service.RemoveTask(testProjectSlug, removedID, testCase.isForced, &fakeSessionKeeper{}, confirmation.answer)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			_, lookupErr := repo.GetTask(testProjectSlug, removedID)
			isRemoved := lookupErr != nil
			if isRemoved != testCase.wantRemoved {
				t.Errorf("expected the task to be removed: %v, got %v", testCase.wantRemoved, isRemoved)
			}

			if gotDependents := taskIDs(confirmation.removal.Dependents); !slices.Equal(gotDependents, testCase.wantDependents) {
				t.Errorf("expected the confirmation to name dependents %v, got %v", testCase.wantDependents, gotDependents)
			}
			if gotChildren := taskIDs(confirmation.removal.Children); !slices.Equal(gotChildren, testCase.wantChildren) {
				t.Errorf("expected the confirmation to name children %v, got %v", testCase.wantChildren, gotChildren)
			}

			for id, want := range testCase.wantLinks {
				stored, err := repo.GetTask(testProjectSlug, id)
				if err != nil {
					t.Fatalf("expected task %s to stay: %v", id, err)
				}
				if !slices.Equal(stored.BlockedBy, want.blockedBy) {
					t.Errorf("expected task %s to be blocked by %v, got %v", ShortID(id), want.blockedBy, stored.BlockedBy)
				}
				if stored.ParentTaskID != want.parentID {
					t.Errorf("expected task %s to belong to %q, got %q", ShortID(id), want.parentID, stored.ParentTaskID)
				}
			}
		})
	}
}

func TestTaskService_RemoveTask_StandsWhenALinkedTaskIsHeld(t *testing.T) {
	const (
		heldID TaskID = "4f2a1b3c-dbe9-4316-8aba-8a67a8f01f8f"
		freeID TaskID = "7e6d5c4b-dbe9-4316-8aba-8a67a8f01f8f"
	)
	held := &Task{ID: heldID, Title: "Wire the repository", BlockedBy: []TaskID{editableTaskID}}
	free := &Task{ID: freeID, Title: "Add the endpoint", ParentTaskID: editableTaskID}
	repo := &fakeTaskRepo{
		tasks:  []*Task{editableTask(), held, free},
		locked: map[TaskID]bool{heldID: true},
	}
	service := NewTaskService(repo, common.NewLogger(""))

	if err := service.RemoveTask(testProjectSlug, editableTaskID, true, &fakeSessionKeeper{}, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := repo.GetTask(testProjectSlug, editableTaskID); err == nil {
		t.Error("expected the task to be gone")
	}
	if !slices.Equal(held.BlockedBy, []TaskID{editableTaskID}) {
		t.Errorf("expected the held task to keep its blockers, got %v", held.BlockedBy)
	}
	if free.ParentTaskID != "" {
		t.Errorf("expected the free task to be ungrouped, got parent %q", free.ParentTaskID)
	}
}

func taskIDs(tasks []*Task) []TaskID {
	var ids []TaskID
	for _, listed := range tasks {
		ids = append(ids, listed.ID)
	}
	return ids
}
