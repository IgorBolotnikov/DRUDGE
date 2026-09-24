package task

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/IgorBolotnikov/DRUDGE/internal/common"
)

// childTask is a task of the backlog grouped under parentID.
func childTask(id TaskID, title string, parentID TaskID) *Task {
	child := backlogTask(id, title, StatusTodo)
	child.ParentTaskID = parentID
	return child
}

func TestTaskService_EditTask_Parent(t *testing.T) {
	cases := []struct {
		name string
		// others are the tasks stored next to the edited one.
		others []*Task
		// storedParent is the parent of the edited task before the edit.
		storedParent   TaskID
		storedBlockers []TaskID
		parent         TaskID
		wantParent     TaskID
		// wantErrText are fragments a refusal must carry. A case without them
		// expects the edit to go through.
		wantErrText []string
	}{
		{
			name:       "setting a parent",
			others:     []*Task{backlogTask(migrationTaskID, "Add the migration", StatusTodo)},
			parent:     migrationTaskID,
			wantParent: migrationTaskID,
		},
		{
			name: "changing a parent",
			others: []*Task{
				backlogTask(migrationTaskID, "Add the migration", StatusTodo),
				backlogTask(docsTaskID, "Write the docs", StatusTodo),
			},
			storedParent: migrationTaskID,
			parent:       docsTaskID,
			wantParent:   docsTaskID,
		},
		{
			name:         "clearing a parent",
			others:       []*Task{backlogTask(migrationTaskID, "Add the migration", StatusTodo)},
			storedParent: migrationTaskID,
			parent:       "",
			wantParent:   "",
		},
		{
			name:         "clearing the parent of a task whose parent is gone",
			storedParent: migrationTaskID,
			parent:       "",
			wantParent:   "",
		},
		{
			name:       "a prefix that resolves",
			others:     []*Task{backlogTask(migrationTaskID, "Add the migration", StatusTodo)},
			parent:     "9c8d",
			wantParent: migrationTaskID,
		},
		{
			name:           "a parent the task is blocked by",
			others:         []*Task{backlogTask(migrationTaskID, "Add the migration", StatusTodo)},
			storedBlockers: []TaskID{migrationTaskID},
			parent:         migrationTaskID,
			wantParent:     migrationTaskID,
		},
		{
			name:       "a parent blocked by the task",
			others:     []*Task{backlogTask(migrationTaskID, "Add the migration", StatusTodo, editableTaskID)},
			parent:     migrationTaskID,
			wantParent: migrationTaskID,
		},
		{
			name: "a prefix that is ambiguous",
			others: []*Task{
				backlogTask(repositoryTaskID, "Wire the repository", StatusTodo),
				backlogTask(endpointTaskID, "Add the endpoint", StatusTodo),
			},
			parent:      "1a2b",
			wantErrText: []string{"1a2b", "Wire the repository", "Add the endpoint"},
		},
		{
			name:        "a parent that names no task",
			others:      []*Task{backlogTask(migrationTaskID, "Add the migration", StatusTodo)},
			parent:      "ffffffff",
			wantErrText: []string{"ffffffff", "not found"},
		},
		{
			name:        "a task naming itself",
			parent:      "006684e3",
			wantErrText: []string{"task 006684e3 cannot be its own parent"},
		},
		{
			name: "a parent that already has a parent",
			others: []*Task{
				backlogTask(migrationTaskID, "Add the migration", StatusTodo),
				childTask(docsTaskID, "Write the docs", migrationTaskID),
			},
			parent:      docsTaskID,
			wantErrText: []string{"task 7e6d5c4b Write the docs belongs to 9c8d7e6f, tasks group one level deep"},
		},
		{
			name: "a task that has children of its own",
			others: []*Task{
				backlogTask(migrationTaskID, "Add the migration", StatusTodo),
				childTask(docsTaskID, "Write the docs", editableTaskID),
			},
			parent:      migrationTaskID,
			wantErrText: []string{"task 006684e3 has tasks belonging to it, tasks group one level deep"},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			stored := editableTask()
			stored.ParentTaskID = testCase.storedParent
			stored.BlockedBy = testCase.storedBlockers
			repo := &fakeTaskRepo{tasks: append([]*Task{stored}, testCase.others...)}
			service := NewTaskService(repo, common.NewLogger(""))

			changes := EditTaskDto{ParentTaskID: &testCase.parent}
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
				if stored.ParentTaskID != testCase.storedParent {
					t.Errorf("expected the parent to stay %q, got %q", testCase.storedParent, stored.ParentTaskID)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if edited.ParentTaskID != testCase.wantParent {
				t.Errorf("expected the parent %q, got %q", testCase.wantParent, edited.ParentTaskID)
			}
			if !reflect.DeepEqual(edited.BlockedBy, testCase.storedBlockers) {
				t.Errorf("expected the blockers to stay %v, got %v", testCase.storedBlockers, edited.BlockedBy)
			}
			if repo.writes != 1 {
				t.Errorf("expected one write, got %d", repo.writes)
			}
		})
	}
}

func TestTaskService_CreateTask_Parent(t *testing.T) {
	cases := []struct {
		name       string
		parent     TaskID
		wantParent TaskID
		// wantErrText is a fragment a refusal must carry. A case without it
		// expects the task to be created.
		wantErrText string
	}{
		{
			name: "no parent",
		},
		{
			name:       "a full id",
			parent:     migrationTaskID,
			wantParent: migrationTaskID,
		},
		{
			name:       "a prefix that resolves",
			parent:     "9c8d",
			wantParent: migrationTaskID,
		},
		{
			name:        "a prefix that is ambiguous",
			parent:      "1a2b",
			wantErrText: "Add the endpoint",
		},
		{
			name:        "a parent that names no task",
			parent:      "ffffffff",
			wantErrText: "ffffffff",
		},
		{
			name:        "a parent that already has a parent",
			parent:      docsTaskID,
			wantErrText: "tasks group one level deep",
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			repo := &fakeTaskRepo{tasks: []*Task{
				backlogTask(migrationTaskID, "Add the migration", StatusTodo),
				backlogTask(repositoryTaskID, "Wire the repository", StatusTodo),
				backlogTask(endpointTaskID, "Add the endpoint", StatusTodo),
				childTask(docsTaskID, "Write the docs", migrationTaskID),
			}}
			service := NewTaskService(repo, common.NewLogger(""))

			created, err := service.CreateTask(CreateTaskDto{
				Title:        "Wire the service",
				ProjectSlug:  testProjectSlug,
				ParentTaskID: testCase.parent,
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
			if created.ParentTaskID != testCase.wantParent {
				t.Errorf("expected the parent %q, got %q", testCase.wantParent, created.ParentTaskID)
			}
		})
	}
}

func TestTaskService_Family(t *testing.T) {
	const missingTaskID TaskID = "00000000-dbe9-4316-8aba-8a67a8f01f8f"
	monday := time.Date(2026, 9, 21, 9, 0, 0, 0, time.UTC)

	migration := backlogTask(migrationTaskID, "Add the migration", StatusTodo)
	olderChild := childTask(docsTaskID, "Write the docs", editableTaskID)
	olderChild.CreatedAt = monday
	newerChild := childTask(repositoryTaskID, "Wire the repository", editableTaskID)
	newerChild.CreatedAt = monday.AddDate(0, 0, 1)

	cases := []struct {
		name   string
		parent TaskID
		others []*Task
		want   Family
	}{
		{
			name:   "an ungrouped task",
			others: []*Task{migration},
			want:   Family{},
		},
		{
			name:   "a task with a parent",
			parent: migrationTaskID,
			others: []*Task{migration},
			want:   Family{ParentID: migrationTaskID, Parent: migration},
		},
		{
			name:   "a task whose parent is gone",
			parent: missingTaskID,
			want:   Family{ParentID: missingTaskID},
		},
		{
			name:   "a task with children, oldest first",
			others: []*Task{migration, newerChild, olderChild},
			want:   Family{Children: []*Task{olderChild, newerChild}},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			member := editableTask()
			member.ParentTaskID = testCase.parent
			repo := &fakeTaskRepo{tasks: append([]*Task{member}, testCase.others...)}
			service := NewTaskService(repo, common.NewLogger(""))

			family, err := service.Family(testProjectSlug, member)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(family, testCase.want) {
				t.Errorf("expected %+v, got %+v", testCase.want, family)
			}
		})
	}
}
