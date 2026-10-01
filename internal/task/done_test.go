package task

import (
	"reflect"
	"strings"
	"testing"

	"github.com/IgorBolotnikov/DRUDGE/internal/common"
)

func TestTaskService_MarkDone(t *testing.T) {
	cases := []struct {
		name     string
		status   TaskStatus
		id       TaskID
		isLocked bool
		// wantErr is part of the refusal, and empty means the task is marked
		// done.
		wantErr string
	}{
		{name: "marks an unmerged task done", status: StatusUnmerged, id: editableTaskID},
		{name: "takes a prefix of the id", status: StatusUnmerged, id: editableTaskID[:8]},
		{name: "refuses a draft task", status: StatusDraft, id: editableTaskID, wantErr: `is "draft"`},
		{name: "refuses a todo task", status: StatusTodo, id: editableTaskID, wantErr: `is "todo"`},
		{name: "refuses an in-progress task", status: StatusInProgress, id: editableTaskID, wantErr: `is "in-progress"`},
		{name: "refuses a fucked-up task", status: StatusFuckedUp, id: editableTaskID, wantErr: `is "fucked-up"`},
		{name: "refuses a task that is already done", status: StatusDone, id: editableTaskID, wantErr: `is "done"`},
		{name: "refuses a task another command holds", status: StatusUnmerged, id: editableTaskID, isLocked: true, wantErr: "another drudge command"},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			stored := editableTask()
			stored.Status = testCase.status
			repo := &fakeTaskRepo{tasks: []*Task{stored}, locked: map[TaskID]bool{stored.ID: testCase.isLocked}}
			progress := &fakeProgress{}
			service := NewTaskService(repo, common.NewLogger("", common.Labels{}), progress, StatusDraft)

			marked, err := service.MarkDone(testProjectSlug, testCase.id)

			if testCase.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if marked.Status != StatusDone || stored.Status != StatusDone {
					t.Errorf("expected the task to be stored %q, got %q", StatusDone, stored.Status)
				}
				wantEvents := []any{TaskMarkedDone{Task: marked}}
				if !reflect.DeepEqual(progress.events, wantEvents) {
					t.Errorf("events = %+v, want %+v", progress.events, wantEvents)
				}
				return
			}

			if err == nil || !strings.Contains(err.Error(), testCase.wantErr) {
				t.Fatalf("expected an error holding %q, got %v", testCase.wantErr, err)
			}
			if repo.writes != 0 {
				t.Errorf("expected a refusal to write nothing, got %d writes", repo.writes)
			}
			if stored.Status != testCase.status {
				t.Errorf("expected the task to stay %q, got %q", testCase.status, stored.Status)
			}
			if len(progress.events) != 0 {
				t.Errorf("expected a refusal to report nothing, got %+v", progress.events)
			}
		})
	}
}
