package task

import (
	"reflect"
	"testing"
	"time"

	"drudge/internal/common"
)

func TestTaskService_NextTask(t *testing.T) {
	const missingTaskID TaskID = "00000000-dbe9-4316-8aba-8a67a8f01f8f"
	monday := time.Date(2026, 9, 21, 9, 0, 0, 0, time.UTC)

	// madeAt is a backlog task created the given number of days after monday.
	madeAt := func(days int, stored *Task) *Task {
		stored.CreatedAt = monday.AddDate(0, 0, days)
		return stored
	}

	cases := []struct {
		name         string
		tasks        []*Task
		wantTask     TaskID
		wantBlockers []TaskID
		// wantBlocked are the blocked todo tasks with the blockers holding
		// each, in the order they are reported.
		wantBlocked []wantBlockedTask
	}{
		{
			name:  "no tasks at all",
			tasks: nil,
		},
		{
			name: "no todo tasks",
			tasks: []*Task{
				backlogTask(docsTaskID, "Write the docs", StatusDraft),
				backlogTask(migrationTaskID, "Add the migration", StatusDone),
			},
		},
		{
			name:     "one candidate",
			tasks:    []*Task{backlogTask(docsTaskID, "Write the docs", StatusTodo)},
			wantTask: docsTaskID,
		},
		{
			name: "several candidates pick the oldest",
			tasks: []*Task{
				madeAt(2, backlogTask(docsTaskID, "Write the docs", StatusTodo)),
				madeAt(0, backlogTask(endpointTaskID, "Add the endpoint", StatusTodo)),
				madeAt(1, backlogTask(migrationTaskID, "Add the migration", StatusTodo)),
			},
			wantTask: endpointTaskID,
		},
		{
			name: "two made at the same moment pick the lowest id",
			tasks: []*Task{
				madeAt(0, backlogTask(endpointTaskID, "Add the endpoint", StatusTodo)),
				madeAt(0, backlogTask(repositoryTaskID, "Wire the repository", StatusTodo)),
			},
			wantTask: repositoryTaskID,
		},
		{
			name: "a todo task with an unfinished blocker is skipped",
			tasks: []*Task{
				madeAt(0, backlogTask(docsTaskID, "Write the docs", StatusTodo, migrationTaskID)),
				madeAt(1, backlogTask(migrationTaskID, "Add the migration", StatusTodo)),
			},
			wantTask: migrationTaskID,
		},
		{
			name: "a parent is an ordinary candidate",
			tasks: []*Task{
				madeAt(1, &Task{ID: docsTaskID, Title: "Write the docs", Status: StatusTodo, ParentTaskID: migrationTaskID}),
				madeAt(0, backlogTask(migrationTaskID, "Add the migration", StatusTodo)),
			},
			wantTask: migrationTaskID,
		},
		{
			name: "a candidate carries its done blockers",
			tasks: []*Task{
				backlogTask(docsTaskID, "Write the docs", StatusTodo, migrationTaskID, repositoryTaskID),
				backlogTask(migrationTaskID, "Add the migration", StatusDone),
				backlogTask(repositoryTaskID, "Wire the repository", StatusDone),
			},
			wantTask:     docsTaskID,
			wantBlockers: []TaskID{migrationTaskID, repositoryTaskID},
		},
		{
			name: "every todo task blocked, oldest first",
			tasks: []*Task{
				madeAt(1, backlogTask(docsTaskID, "Write the docs", StatusTodo, migrationTaskID, repositoryTaskID)),
				madeAt(0, backlogTask(endpointTaskID, "Add the endpoint", StatusTodo, missingTaskID)),
				backlogTask(migrationTaskID, "Add the migration", StatusInProgress),
				backlogTask(repositoryTaskID, "Wire the repository", StatusDone),
			},
			wantBlocked: []wantBlockedTask{
				{id: endpointTaskID, holding: []TaskID{missingTaskID}},
				{id: docsTaskID, holding: []TaskID{migrationTaskID}},
			},
		},
		{
			name: "a cycle reports both tasks as blocked",
			tasks: []*Task{
				madeAt(0, backlogTask(docsTaskID, "Write the docs", StatusTodo, endpointTaskID)),
				madeAt(1, backlogTask(endpointTaskID, "Add the endpoint", StatusTodo, docsTaskID)),
			},
			wantBlocked: []wantBlockedTask{
				{id: docsTaskID, holding: []TaskID{endpointTaskID}},
				{id: endpointTaskID, holding: []TaskID{docsTaskID}},
			},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			service := NewTaskService(&fakeTaskRepo{tasks: testCase.tasks}, common.NewLogger(""))

			pick, err := service.NextTask(testProjectSlug)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			var gotTask TaskID
			if pick.Task != nil {
				gotTask = pick.Task.ID
			}
			if gotTask != testCase.wantTask {
				t.Errorf("expected task %q, got %q", testCase.wantTask, gotTask)
			}

			var gotBlockers []TaskID
			for _, blocker := range pick.Blockers {
				gotBlockers = append(gotBlockers, blocker.ID)
			}
			if !reflect.DeepEqual(gotBlockers, testCase.wantBlockers) {
				t.Errorf("expected blockers %v, got %v", testCase.wantBlockers, gotBlockers)
			}

			var gotBlocked []wantBlockedTask
			for _, blocked := range pick.Blocked {
				gotBlocked = append(gotBlocked, wantBlockedTask{id: blocked.Task.ID, holding: blocked.Holding})
			}
			if !reflect.DeepEqual(gotBlocked, testCase.wantBlocked) {
				t.Errorf("expected blocked %v, got %v", testCase.wantBlocked, gotBlocked)
			}
		})
	}
}

type wantBlockedTask struct {
	id      TaskID
	holding []TaskID
}
