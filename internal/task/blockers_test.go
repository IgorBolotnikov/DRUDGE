package task

import (
	"reflect"
	"slices"
	"strings"
	"testing"

	"drudge/internal/common"
)

// Tasks of a backlog the blocker tests link together. editableTask is the one
// an edit changes.
const (
	migrationTaskID  TaskID = "9c8d7e6f-dbe9-4316-8aba-8a67a8f01f8f"
	repositoryTaskID TaskID = "1a2b3c4d-dbe9-4316-8aba-8a67a8f01f8f"
	endpointTaskID   TaskID = "1a2b9999-dbe9-4316-8aba-8a67a8f01f8f"
	docsTaskID       TaskID = "7e6d5c4b-dbe9-4316-8aba-8a67a8f01f8f"
)

// backlogTask is a task of the backlog blocked by the given tasks.
func backlogTask(id TaskID, title string, status TaskStatus, blockedBy ...TaskID) *Task {
	return &Task{ID: id, Title: title, Status: status, ProjectSlug: testProjectSlug, BlockedBy: blockedBy}
}

func TestTaskService_EditTask_BlockedBy(t *testing.T) {
	cases := []struct {
		name string
		// others are the tasks stored next to the edited one.
		others []*Task
		// storedBlockers are what the edited task is blocked by before the edit.
		storedBlockers []TaskID
		blockedBy      []TaskID
		wantBlockedBy  []TaskID
		// wantErrText are fragments a refusal must carry. A case without them
		// expects the edit to go through.
		wantErrText []string
	}{
		{
			name:          "setting a list",
			others:        []*Task{backlogTask(migrationTaskID, "Add the migration", StatusDone)},
			blockedBy:     []TaskID{migrationTaskID},
			wantBlockedBy: []TaskID{migrationTaskID},
		},
		{
			name: "replacing a list",
			others: []*Task{
				backlogTask(migrationTaskID, "Add the migration", StatusDone),
				backlogTask(docsTaskID, "Write the docs", StatusTodo),
			},
			storedBlockers: []TaskID{migrationTaskID},
			blockedBy:      []TaskID{docsTaskID},
			wantBlockedBy:  []TaskID{docsTaskID},
		},
		{
			name:           "clearing a list",
			others:         []*Task{backlogTask(migrationTaskID, "Add the migration", StatusDone)},
			storedBlockers: []TaskID{migrationTaskID},
			blockedBy:      []TaskID{},
			wantBlockedBy:  nil,
		},
		{
			name:          "a prefix that resolves",
			others:        []*Task{backlogTask(migrationTaskID, "Add the migration", StatusDone)},
			blockedBy:     []TaskID{"9c8d"},
			wantBlockedBy: []TaskID{migrationTaskID},
		},
		{
			name: "duplicates collapsing",
			others: []*Task{
				backlogTask(migrationTaskID, "Add the migration", StatusDone),
				backlogTask(docsTaskID, "Write the docs", StatusTodo),
			},
			blockedBy:     []TaskID{migrationTaskID, docsTaskID, "9c8d7e6f", migrationTaskID},
			wantBlockedBy: []TaskID{migrationTaskID, docsTaskID},
		},
		{
			name: "a chain that is deep but legal",
			others: []*Task{
				backlogTask(docsTaskID, "Write the docs", StatusTodo, endpointTaskID),
				backlogTask(endpointTaskID, "Add the endpoint", StatusTodo, repositoryTaskID),
				backlogTask(repositoryTaskID, "Wire the repository", StatusTodo, migrationTaskID),
				backlogTask(migrationTaskID, "Add the migration", StatusTodo),
			},
			blockedBy:     []TaskID{docsTaskID},
			wantBlockedBy: []TaskID{docsTaskID},
		},
		{
			name: "a chain that meets itself without reaching the edited task",
			others: []*Task{
				backlogTask(docsTaskID, "Write the docs", StatusTodo, migrationTaskID),
				backlogTask(migrationTaskID, "Add the migration", StatusTodo, docsTaskID),
			},
			blockedBy:     []TaskID{docsTaskID},
			wantBlockedBy: []TaskID{docsTaskID},
		},
		{
			name:        "a prefix that matches nothing",
			others:      []*Task{backlogTask(migrationTaskID, "Add the migration", StatusDone)},
			blockedBy:   []TaskID{"ffffffff"},
			wantErrText: []string{"ffffffff", "not found"},
		},
		{
			name: "a prefix that is ambiguous",
			others: []*Task{
				backlogTask(repositoryTaskID, "Wire the repository", StatusTodo),
				backlogTask(endpointTaskID, "Add the endpoint", StatusTodo),
			},
			blockedBy:   []TaskID{"1a2b"},
			wantErrText: []string{"1a2b", "Wire the repository", "Add the endpoint"},
		},
		{
			name:        "a self-reference",
			blockedBy:   []TaskID{"006684e3"},
			wantErrText: []string{"task 006684e3 cannot be blocked by 006684e3, that would make a cycle:\n  006684e3  Fix login"},
		},
		{
			name:        "a two-task cycle",
			others:      []*Task{backlogTask(migrationTaskID, "Add the migration", StatusTodo, editableTaskID)},
			blockedBy:   []TaskID{migrationTaskID},
			wantErrText: []string{"task 006684e3 cannot be blocked by 9c8d7e6f, that would make a cycle:\n  9c8d7e6f  Add the migration\n  006684e3  Fix login"},
		},
		{
			name: "a longer cycle",
			others: []*Task{
				backlogTask(migrationTaskID, "Add the migration", StatusTodo, docsTaskID, repositoryTaskID),
				backlogTask(docsTaskID, "Write the docs", StatusTodo),
				backlogTask(repositoryTaskID, "Wire the repository", StatusTodo, editableTaskID),
			},
			blockedBy: []TaskID{migrationTaskID},
			wantErrText: []string{
				"task 006684e3 cannot be blocked by 9c8d7e6f, that would make a cycle:\n" +
					"  9c8d7e6f  Add the migration\n" +
					"  1a2b3c4d  Wire the repository\n" +
					"  006684e3  Fix login",
			},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			stored := editableTask()
			stored.BlockedBy = testCase.storedBlockers
			repo := &fakeTaskRepo{tasks: append([]*Task{stored}, testCase.others...)}
			service := NewTaskService(repo, common.NewLogger(""))

			changes := EditTaskDto{BlockedBy: &testCase.blockedBy}
			edited, err := service.EditTask(testProjectSlug, editableTaskID, changes, &fakeSessionGuard{})

			if testCase.wantErrText != nil {
				if err == nil {
					t.Fatal("expected the edit to be refused")
				}
				for _, want := range testCase.wantErrText {
					if !strings.Contains(err.Error(), want) {
						t.Errorf("expected the error to carry %q, got %q", want, err)
					}
				}
				if repo.writes != 0 {
					t.Errorf("expected a refused edit to write nothing, got %d writes", repo.writes)
				}
				if !slices.Equal(stored.BlockedBy, testCase.storedBlockers) {
					t.Errorf("expected the blockers to stay %v, got %v", testCase.storedBlockers, stored.BlockedBy)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(edited.BlockedBy, testCase.wantBlockedBy) {
				t.Errorf("expected the blockers %#v, got %#v", testCase.wantBlockedBy, edited.BlockedBy)
			}
			if repo.writes != 1 {
				t.Errorf("expected one write, got %d", repo.writes)
			}
		})
	}
}

func TestTaskService_CreateTask_BlockedBy(t *testing.T) {
	cases := []struct {
		name          string
		blockedBy     []TaskID
		wantBlockedBy []TaskID
		// wantErrText is a fragment a refusal must carry. A case without it
		// expects the task to be created.
		wantErrText string
	}{
		{
			name: "no blockers",
		},
		{
			name:          "full ids and prefixes",
			blockedBy:     []TaskID{migrationTaskID, "7e6d"},
			wantBlockedBy: []TaskID{migrationTaskID, docsTaskID},
		},
		{
			name:          "duplicates collapsing",
			blockedBy:     []TaskID{"9c8d", migrationTaskID},
			wantBlockedBy: []TaskID{migrationTaskID},
		},
		{
			name:        "a prefix that matches nothing",
			blockedBy:   []TaskID{"ffffffff"},
			wantErrText: "ffffffff",
		},
		{
			name:        "a prefix that is ambiguous",
			blockedBy:   []TaskID{"1a2b"},
			wantErrText: "Add the endpoint",
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			repo := &fakeTaskRepo{tasks: []*Task{
				backlogTask(migrationTaskID, "Add the migration", StatusDone),
				backlogTask(repositoryTaskID, "Wire the repository", StatusTodo),
				backlogTask(endpointTaskID, "Add the endpoint", StatusTodo),
				backlogTask(docsTaskID, "Write the docs", StatusTodo),
			}}
			service := NewTaskService(repo, common.NewLogger(""))

			created, err := service.CreateTask(CreateTaskDto{
				Title:       "Wire the service",
				ProjectSlug: testProjectSlug,
				BlockedBy:   testCase.blockedBy,
			})

			if testCase.wantErrText != "" {
				if err == nil {
					t.Fatal("expected the task to be refused")
				}
				if !strings.Contains(err.Error(), testCase.wantErrText) {
					t.Errorf("expected the error to carry %q, got %q", testCase.wantErrText, err)
				}
				if len(repo.tasks) != 4 {
					t.Errorf("expected a refused task to be left unwritten, got %d tasks", len(repo.tasks))
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(created.BlockedBy, testCase.wantBlockedBy) {
				t.Errorf("expected the blockers %#v, got %#v", testCase.wantBlockedBy, created.BlockedBy)
			}
		})
	}
}

func TestTaskService_Blockers(t *testing.T) {
	migration := backlogTask(migrationTaskID, "Add the migration", StatusDone)
	docs := backlogTask(docsTaskID, "Write the docs", StatusTodo)
	const missingTaskID TaskID = "00000000-dbe9-4316-8aba-8a67a8f01f8f"

	cases := []struct {
		name      string
		blockedBy []TaskID
		want      []Blocker
	}{
		{
			name: "a task blocked by nothing",
			want: []Blocker{},
		},
		{
			name:      "blockers in the order the task names them",
			blockedBy: []TaskID{docsTaskID, migrationTaskID},
			want:      []Blocker{{ID: docsTaskID, Task: docs}, {ID: migrationTaskID, Task: migration}},
		},
		{
			name:      "a blocker naming no task",
			blockedBy: []TaskID{missingTaskID, migrationTaskID},
			want:      []Blocker{{ID: missingTaskID}, {ID: migrationTaskID, Task: migration}},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			dependent := editableTask()
			dependent.BlockedBy = testCase.blockedBy
			repo := &fakeTaskRepo{tasks: []*Task{dependent, migration, docs}}
			service := NewTaskService(repo, common.NewLogger(""))

			blockers, err := service.Blockers(testProjectSlug, dependent)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(blockers, testCase.want) {
				t.Errorf("expected %+v, got %+v", testCase.want, blockers)
			}
		})
	}
}

func TestTaskService_EditTask_BlockAndUnblock(t *testing.T) {
	backlog := func() []*Task {
		return []*Task{
			backlogTask(migrationTaskID, "Add the migration", StatusDone),
			backlogTask(repositoryTaskID, "Wire the repository", StatusTodo),
			backlogTask(endpointTaskID, "Add the endpoint", StatusTodo),
			backlogTask(docsTaskID, "Write the docs", StatusTodo),
		}
	}

	cases := []struct {
		name string
		// others replaces the backlog stored next to the edited task when set.
		others         []*Task
		storedBlockers []TaskID
		changes        EditTaskDto
		wantBlockedBy  []TaskID
		// wantErrText are fragments a refusal must carry. A case without them
		// expects the edit to go through.
		wantErrText []string
	}{
		{
			name:          "adding to an empty list",
			changes:       EditTaskDto{Block: &[]TaskID{migrationTaskID}},
			wantBlockedBy: []TaskID{migrationTaskID},
		},
		{
			name:           "adding to a full list",
			storedBlockers: []TaskID{migrationTaskID, repositoryTaskID},
			changes:        EditTaskDto{Block: &[]TaskID{docsTaskID, endpointTaskID}},
			wantBlockedBy:  []TaskID{migrationTaskID, repositoryTaskID, docsTaskID, endpointTaskID},
		},
		{
			name:           "adding an id already on the list",
			storedBlockers: []TaskID{migrationTaskID},
			changes:        EditTaskDto{Block: &[]TaskID{"9c8d", docsTaskID}},
			wantBlockedBy:  []TaskID{migrationTaskID, docsTaskID},
		},
		{
			name:          "adding by a prefix",
			changes:       EditTaskDto{Block: &[]TaskID{"7e6d"}},
			wantBlockedBy: []TaskID{docsTaskID},
		},
		{
			name:        "adding a prefix that matches nothing",
			changes:     EditTaskDto{Block: &[]TaskID{"ffffffff"}},
			wantErrText: []string{"ffffffff", "not found"},
		},
		{
			name:        "adding a prefix that is ambiguous",
			changes:     EditTaskDto{Block: &[]TaskID{"1a2b"}},
			wantErrText: []string{"1a2b", "Wire the repository", "Add the endpoint"},
		},
		{
			name: "adding a blocker that makes a cycle",
			others: []*Task{
				backlogTask(migrationTaskID, "Add the migration", StatusTodo, editableTaskID),
			},
			changes:     EditTaskDto{Block: &[]TaskID{migrationTaskID}},
			wantErrText: []string{"task 006684e3 cannot be blocked by 9c8d7e6f, that would make a cycle:\n  9c8d7e6f  Add the migration\n  006684e3  Fix login"},
		},
		{
			name:        "adding nothing",
			changes:     EditTaskDto{Block: &[]TaskID{}},
			wantErrText: []string{"name at least one task to add"},
		},
		{
			name:           "removing one of several",
			storedBlockers: []TaskID{migrationTaskID, repositoryTaskID, docsTaskID},
			changes:        EditTaskDto{Unblock: &[]TaskID{repositoryTaskID}},
			wantBlockedBy:  []TaskID{migrationTaskID, docsTaskID},
		},
		{
			name:           "removing the last one",
			storedBlockers: []TaskID{migrationTaskID},
			changes:        EditTaskDto{Unblock: &[]TaskID{migrationTaskID}},
			wantBlockedBy:  nil,
		},
		{
			name:           "removing by a prefix",
			storedBlockers: []TaskID{migrationTaskID, docsTaskID},
			changes:        EditTaskDto{Unblock: &[]TaskID{"7e6d"}},
			wantBlockedBy:  []TaskID{migrationTaskID},
		},
		{
			name:           "removing by a prefix that is unique among the blockers",
			storedBlockers: []TaskID{repositoryTaskID},
			changes:        EditTaskDto{Unblock: &[]TaskID{"1a2b"}},
			wantBlockedBy:  nil,
		},
		{
			name:           "removing a blocker that names no stored task",
			others:         []*Task{},
			storedBlockers: []TaskID{migrationTaskID},
			changes:        EditTaskDto{Unblock: &[]TaskID{"9c8d"}},
			wantBlockedBy:  nil,
		},
		{
			name:           "removing one that is not there",
			storedBlockers: []TaskID{migrationTaskID},
			changes:        EditTaskDto{Unblock: &[]TaskID{docsTaskID}},
			wantErrText:    []string{"task 006684e3 is not blocked by 7e6d5c4b"},
		},
		{
			name:           "removing a prefix that matches nothing",
			storedBlockers: []TaskID{migrationTaskID},
			changes:        EditTaskDto{Unblock: &[]TaskID{"ffffffff"}},
			wantErrText:    []string{"ffffffff", "not found"},
		},
		{
			name:           "removing a prefix that is ambiguous among the blockers",
			storedBlockers: []TaskID{repositoryTaskID, endpointTaskID},
			changes:        EditTaskDto{Unblock: &[]TaskID{"1a2b"}},
			wantErrText:    []string{"1a2b", "Wire the repository", "Add the endpoint"},
		},
		{
			name:        "removing nothing",
			changes:     EditTaskDto{Unblock: &[]TaskID{}},
			wantErrText: []string{"name at least one task to remove"},
		},
		{
			name: "replacing the list and adding",
			changes: EditTaskDto{
				BlockedBy: &[]TaskID{migrationTaskID},
				Block:     &[]TaskID{docsTaskID},
			},
			wantErrText: []string{"the blocker list", "the blockers to add"},
		},
		{
			name: "replacing the list and removing",
			changes: EditTaskDto{
				BlockedBy: &[]TaskID{migrationTaskID},
				Unblock:   &[]TaskID{docsTaskID},
			},
			wantErrText: []string{"the blocker list", "the blockers to remove"},
		},
		{
			name: "adding and removing",
			changes: EditTaskDto{
				Block:   &[]TaskID{migrationTaskID},
				Unblock: &[]TaskID{docsTaskID},
			},
			wantErrText: []string{"the blockers to add", "the blockers to remove"},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			stored := editableTask()
			stored.BlockedBy = testCase.storedBlockers
			others := testCase.others
			if others == nil {
				others = backlog()
			}
			repo := &fakeTaskRepo{tasks: append([]*Task{stored}, others...)}
			service := NewTaskService(repo, common.NewLogger(""))

			edited, err := service.EditTask(testProjectSlug, editableTaskID, testCase.changes, &fakeSessionGuard{})

			if testCase.wantErrText != nil {
				if err == nil {
					t.Fatal("expected the edit to be refused")
				}
				for _, want := range testCase.wantErrText {
					if !strings.Contains(err.Error(), want) {
						t.Errorf("expected the error to carry %q, got %q", want, err)
					}
				}
				if repo.writes != 0 {
					t.Errorf("expected a refused edit to write nothing, got %d writes", repo.writes)
				}
				if !slices.Equal(stored.BlockedBy, testCase.storedBlockers) {
					t.Errorf("expected the blockers to stay %v, got %v", testCase.storedBlockers, stored.BlockedBy)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(edited.BlockedBy, testCase.wantBlockedBy) {
				t.Errorf("expected the blockers %#v, got %#v", testCase.wantBlockedBy, edited.BlockedBy)
			}
			if repo.writes != 1 {
				t.Errorf("expected one write, got %d", repo.writes)
			}
		})
	}
}
