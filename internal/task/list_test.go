package task

import (
	"slices"
	"testing"
	"time"

	"drudge/internal/common"
)

func TestTaskService_ListTasks(t *testing.T) {
	monday := time.Date(2026, 9, 21, 9, 0, 0, 0, time.UTC)
	stored := []*Task{
		{ID: "oldest-todo", Status: StatusTodo, TicketID: "T-1", CreatedAt: monday},
		{ID: "newest-draft", Status: StatusDraft, CreatedAt: monday.Add(48 * time.Hour)},
		{ID: "middle-todo", Status: StatusTodo, TicketID: "T-2", CreatedAt: monday.Add(24 * time.Hour)},
	}

	cases := []struct {
		name    string
		filter  ListTasksFilter
		wantIDs []TaskID
	}{
		{
			name:    "lists every task newest first",
			wantIDs: []TaskID{"newest-draft", "middle-todo", "oldest-todo"},
		},
		{
			name:    "keeps the tasks in a status",
			filter:  ListTasksFilter{Status: pointerTo(StatusTodo)},
			wantIDs: []TaskID{"middle-todo", "oldest-todo"},
		},
		{
			name:    "keeps the tasks of a ticket",
			filter:  ListTasksFilter{TicketID: pointerTo("T-1")},
			wantIDs: []TaskID{"oldest-todo"},
		},
		{
			name:    "an empty ticket keeps the tasks with no ticket",
			filter:  ListTasksFilter{TicketID: pointerTo("")},
			wantIDs: []TaskID{"newest-draft"},
		},
		{
			name:    "combines the status and the ticket",
			filter:  ListTasksFilter{Status: pointerTo(StatusTodo), TicketID: pointerTo("T-2")},
			wantIDs: []TaskID{"middle-todo"},
		},
		{
			name:    "a status no task has keeps nothing",
			filter:  ListTasksFilter{Status: pointerTo(StatusDone)},
			wantIDs: nil,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			repo := &mockRepo{
				listTasksFn: func(string) ([]*Task, error) {
					return slices.Clone(stored), nil
				},
			}
			service := NewTaskService(repo, common.NewLogger(""))

			listed, err := service.ListTasks("test", testCase.filter)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			var gotIDs []TaskID
			for _, listedTask := range listed {
				gotIDs = append(gotIDs, listedTask.ID)
			}
			if !slices.Equal(gotIDs, testCase.wantIDs) {
				t.Errorf("got %v, want %v", gotIDs, testCase.wantIDs)
			}
		})
	}
}
