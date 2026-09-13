package task

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"drudge/internal/common"
)

const testProjectSlug = "demo"

// fakeTaskRepo holds tasks in memory. A lookup and an update both hand out the
// stored task, so a test reads what an edit wrote off the value it set up.
type fakeTaskRepo struct {
	tasks []*Task
	// locked are the tasks another command holds the lock on. A try update of
	// one of them stores nothing.
	locked map[TaskID]bool
	writes int
}

func (repo *fakeTaskRepo) CreateTask(dto CreateTaskDto) (*Task, error) {
	return nil, errors.New("CreateTask should not be called")
}

func (repo *fakeTaskRepo) ListTasks(projectSlug string) ([]*Task, error) {
	return repo.tasks, nil
}

func (repo *fakeTaskRepo) GetTask(projectSlug string, id TaskID) (*Task, error) {
	for _, candidate := range repo.tasks {
		if candidate.ID == id {
			return candidate, nil
		}
	}
	return nil, NotFoundError(string(id))
}

func (repo *fakeTaskRepo) FindTask(projectSlug string, fullOrPartialID string) (*Task, error) {
	var matches []*Task
	for _, candidate := range repo.tasks {
		if candidate.ID == TaskID(fullOrPartialID) {
			return candidate, nil
		}
		if strings.HasPrefix(string(candidate.ID), fullOrPartialID) {
			matches = append(matches, candidate)
		}
	}

	switch len(matches) {
	case 0:
		return nil, NotFoundError(fullOrPartialID)
	case 1:
		return matches[0], nil
	default:
		found := make([]IDMatch, 0, len(matches))
		for _, match := range matches {
			found = append(found, IDMatch{ID: match.ID, Title: match.Title})
		}
		return nil, AmbiguousIDError(fullOrPartialID, found)
	}
}

func (repo *fakeTaskRepo) UpdateTask(projectSlug string, id TaskID, change func(*Task) error) error {
	stored, err := repo.GetTask(projectSlug, id)
	if err != nil {
		return err
	}

	if err := change(stored); err != nil {
		if errors.Is(err, ErrTaskUnchanged) {
			return nil
		}
		return err
	}

	repo.writes++
	return nil
}

func (repo *fakeTaskRepo) TryUpdateTask(projectSlug string, id TaskID, change func(*Task) error) (bool, error) {
	if repo.locked[id] {
		return false, nil
	}
	return true, repo.UpdateTask(projectSlug, id, change)
}

// fakeSessionGuard stands for the live Session check the drudger service does.
type fakeSessionGuard struct {
	refusal error
	asked   TaskID
}

func (guard *fakeSessionGuard) RefuseWhileWorking(projectSlug string, taskToChange *Task) error {
	guard.asked = taskToChange.ID
	return guard.refusal
}

const editableTaskID TaskID = "006684e3-dbe9-4316-8aba-8a67a8f01f8f"

func editableTask() *Task {
	return &Task{
		ID:          editableTaskID,
		Title:       "Fix login",
		Description: "SSO is broken",
		Status:      StatusDraft,
		TicketID:    "R-003-10",
		ProjectSlug: testProjectSlug,
	}
}

func pointerTo[Value any](value Value) *Value {
	return &value
}

func TestTaskService_EditTask(t *testing.T) {
	cases := []struct {
		name    string
		changes EditTaskDto
		// expect turns a copy of the task into what the edit should leave.
		expect func(*Task)
	}{
		{
			name:    "the title",
			changes: EditTaskDto{Title: pointerTo("Fix logout")},
			expect:  func(edited *Task) { edited.Title = "Fix logout" },
		},
		{
			name:    "the description",
			changes: EditTaskDto{Description: pointerTo("SSO logs nobody out")},
			expect:  func(edited *Task) { edited.Description = "SSO logs nobody out" },
		},
		{
			name:    "the description emptied",
			changes: EditTaskDto{Description: pointerTo("")},
			expect:  func(edited *Task) { edited.Description = "" },
		},
		{
			name:    "the ticket",
			changes: EditTaskDto{TicketID: pointerTo("R-004-01")},
			expect:  func(edited *Task) { edited.TicketID = "R-004-01" },
		},
		{
			name:    "the ticket cleared",
			changes: EditTaskDto{TicketID: pointerTo("")},
			expect:  func(edited *Task) { edited.TicketID = "" },
		},
		{
			name:    "the status",
			changes: EditTaskDto{Status: pointerTo(StatusTodo)},
			expect:  func(edited *Task) { edited.Status = StatusTodo },
		},
		{
			name: "every editable field at once",
			changes: EditTaskDto{
				Title:       pointerTo("Fix logout"),
				Description: pointerTo("SSO logs nobody out"),
				TicketID:    pointerTo("R-004-01"),
				Status:      pointerTo(StatusTodo),
			},
			expect: func(edited *Task) {
				edited.Title = "Fix logout"
				edited.Description = "SSO logs nobody out"
				edited.TicketID = "R-004-01"
				edited.Status = StatusTodo
			},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			stored := editableTask()
			wanted := editableTask()
			testCase.expect(wanted)

			repo := &fakeTaskRepo{tasks: []*Task{stored}}
			service := NewTaskService(repo, common.NewLogger(""))

			edited, err := service.EditTask(testProjectSlug, editableTaskID, testCase.changes, &fakeSessionGuard{})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(*edited, *wanted) {
				t.Errorf("expected %+v, got %+v", *wanted, *edited)
			}
			if !reflect.DeepEqual(*stored, *wanted) {
				t.Errorf("expected the stored task to be %+v, got %+v", *wanted, *stored)
			}
			if repo.writes != 1 {
				t.Errorf("expected one write, got %d", repo.writes)
			}
		})
	}
}

func TestTaskService_EditTask_ResolvesAnIDPrefix(t *testing.T) {
	stored := editableTask()
	repo := &fakeTaskRepo{tasks: []*Task{stored}}
	service := NewTaskService(repo, common.NewLogger(""))

	edited, err := service.EditTask(testProjectSlug, "006684e3", EditTaskDto{Status: pointerTo(StatusTodo)}, &fakeSessionGuard{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if edited.ID != editableTaskID {
		t.Errorf("expected the edit to resolve to task %q, got %q", editableTaskID, edited.ID)
	}
}

func TestTaskService_EditTask_RefusesAStatusDrudgeMaintains(t *testing.T) {
	// These statuses describe a Session. Setting one by hand makes the record
	// claim a run that never happened.
	for _, managed := range []TaskStatus{StatusInProgress, StatusFuckedUp, StatusDone} {
		t.Run(string(managed), func(t *testing.T) {
			stored := editableTask()
			repo := &fakeTaskRepo{tasks: []*Task{stored}}
			service := NewTaskService(repo, common.NewLogger(""))

			_, err := service.EditTask(testProjectSlug, editableTaskID, EditTaskDto{Status: pointerTo(managed)}, &fakeSessionGuard{})
			if err == nil {
				t.Fatalf("expected %q to be refused", managed)
			}
			if !strings.Contains(err.Error(), string(managed)) {
				t.Errorf("expected the error to name %q, got %q", managed, err)
			}
			if repo.writes != 0 {
				t.Errorf("expected a refused edit to write nothing, got %d writes", repo.writes)
			}
			if stored.Status != StatusDraft {
				t.Errorf("expected the task to stay %q, got %q", StatusDraft, stored.Status)
			}
		})
	}
}

func TestTaskService_EditTask_TakesAStatusDrudgeMaintainsUnderForce(t *testing.T) {
	for _, managed := range []TaskStatus{StatusInProgress, StatusFuckedUp, StatusDone} {
		t.Run(string(managed), func(t *testing.T) {
			stored := editableTask()
			repo := &fakeTaskRepo{tasks: []*Task{stored}}
			service := NewTaskService(repo, common.NewLogger(""))

			changes := EditTaskDto{Status: pointerTo(managed), AllowManagedStatus: true}
			if _, err := service.EditTask(testProjectSlug, editableTaskID, changes, &fakeSessionGuard{}); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if stored.Status != managed {
				t.Errorf("expected the task to be %q, got %q", managed, stored.Status)
			}
		})
	}
}

func TestTaskService_EditTask_RefusesAnUnknownStatus(t *testing.T) {
	stored := editableTask()
	repo := &fakeTaskRepo{tasks: []*Task{stored}}
	service := NewTaskService(repo, common.NewLogger(""))

	changes := EditTaskDto{Status: pointerTo(TaskStatus("almost-done")), AllowManagedStatus: true}
	_, err := service.EditTask(testProjectSlug, editableTaskID, changes, &fakeSessionGuard{})
	if err == nil {
		t.Fatal("expected an unknown status to be refused")
	}
	for _, want := range []string{"almost-done", string(StatusTodo)} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("expected the error to name %q, got %q", want, err)
		}
	}
	if repo.writes != 0 {
		t.Errorf("expected a refused edit to write nothing, got %d writes", repo.writes)
	}
}

func TestTaskService_EditTask_RefusesABlankTitle(t *testing.T) {
	stored := editableTask()
	repo := &fakeTaskRepo{tasks: []*Task{stored}}
	service := NewTaskService(repo, common.NewLogger(""))

	_, err := service.EditTask(testProjectSlug, editableTaskID, EditTaskDto{Title: pointerTo("  ")}, &fakeSessionGuard{})
	if err == nil {
		t.Fatal("expected a blank title to be refused")
	}
	if stored.Title != "Fix login" {
		t.Errorf("expected the title to be left alone, got %q", stored.Title)
	}
}

func TestTaskService_EditTask_RefusesAnEditThatChangesNothing(t *testing.T) {
	stored := editableTask()
	repo := &fakeTaskRepo{tasks: []*Task{stored}}
	service := NewTaskService(repo, common.NewLogger(""))

	_, err := service.EditTask(testProjectSlug, editableTaskID, EditTaskDto{}, &fakeSessionGuard{})
	if !errors.Is(err, ErrNoChanges) {
		t.Fatalf("expected %v, got %v", ErrNoChanges, err)
	}
	if repo.writes != 0 {
		t.Errorf("expected nothing to be written, got %d writes", repo.writes)
	}
}

func TestTaskService_EditTask_RefusesAnEmptyID(t *testing.T) {
	repo := &fakeTaskRepo{tasks: []*Task{editableTask()}}
	service := NewTaskService(repo, common.NewLogger(""))

	_, err := service.EditTask(testProjectSlug, "", EditTaskDto{Title: pointerTo("Fix logout")}, &fakeSessionGuard{})
	if !errors.Is(err, ErrNoTaskID) {
		t.Fatalf("expected %v, got %v", ErrNoTaskID, err)
	}
}

func TestTaskService_EditTask_SurfacesTheLookupFailure(t *testing.T) {
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

			_, err := service.EditTask(testProjectSlug, testCase.id, EditTaskDto{Title: pointerTo("Fix signup")}, &fakeSessionGuard{})
			if err == nil {
				t.Fatal("expected the lookup failure to surface")
			}
			if !strings.Contains(err.Error(), testCase.wantText) {
				t.Errorf("expected the error to mention %q, got %q", testCase.wantText, err)
			}
			if repo.writes != 0 {
				t.Errorf("expected nothing to be written, got %d writes", repo.writes)
			}
		})
	}
}

func TestTaskService_EditTask_RefusesATaskAnAgentIsWorkingOn(t *testing.T) {
	stored := editableTask()
	stored.Status = StatusInProgress
	repo := &fakeTaskRepo{tasks: []*Task{stored}}
	service := NewTaskService(repo, common.NewLogger(""))

	guard := &fakeSessionGuard{refusal: errors.New("Drudger 1 (drudge-claude-demo-1) is still working on this task")}

	_, err := service.EditTask(testProjectSlug, editableTaskID, EditTaskDto{Title: pointerTo("Fix logout")}, guard)
	if err == nil {
		t.Fatal("expected an edit of a task with a live Session to be refused")
	}
	if !strings.Contains(err.Error(), "drudge-claude-demo-1") {
		t.Errorf("expected the error to name the Drudger, got %q", err)
	}
	if guard.asked != editableTaskID {
		t.Errorf("expected the guard to be asked about task %q, got %q", editableTaskID, guard.asked)
	}
	if repo.writes != 0 {
		t.Errorf("expected a refused edit to write nothing, got %d writes", repo.writes)
	}
	if stored.Title != "Fix login" {
		t.Errorf("expected the task to be left alone, got title %q", stored.Title)
	}
}

func TestTaskService_EditTask_TakesATaskWhoseSessionHasFinished(t *testing.T) {
	stored := editableTask()
	stored.Status = StatusFuckedUp
	repo := &fakeTaskRepo{tasks: []*Task{stored}}
	service := NewTaskService(repo, common.NewLogger(""))

	changes := EditTaskDto{Status: pointerTo(StatusTodo)}
	if _, err := service.EditTask(testProjectSlug, editableTaskID, changes, &fakeSessionGuard{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stored.Status != StatusTodo {
		t.Errorf("expected the task to be %q, got %q", StatusTodo, stored.Status)
	}
}

func TestTaskService_EditTask_ReportsAnotherCommandHoldingTheTask(t *testing.T) {
	stored := editableTask()
	repo := &fakeTaskRepo{
		tasks:  []*Task{stored},
		locked: map[TaskID]bool{editableTaskID: true},
	}
	service := NewTaskService(repo, common.NewLogger(""))

	_, err := service.EditTask(testProjectSlug, editableTaskID, EditTaskDto{Title: pointerTo("Fix logout")}, &fakeSessionGuard{})
	if err == nil {
		t.Fatal("expected an edit of a locked task to be refused")
	}
	if !strings.Contains(err.Error(), string(editableTaskID)) {
		t.Errorf("expected the error to name the task, got %q", err)
	}
}
