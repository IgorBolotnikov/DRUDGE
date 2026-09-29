package task

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/IgorBolotnikov/DRUDGE/internal/common"
)

func TestTaskService_ListTasks(t *testing.T) {
	monday := time.Date(2026, 9, 21, 9, 0, 0, 0, time.UTC)
	stored := func(id TaskID, status TaskStatus, ticketID string, daysAfterMonday int) *Task {
		return &Task{ID: id, Status: status, TicketID: ticketID, CreatedAt: monday.AddDate(0, 0, daysAfterMonday)}
	}
	childOf := func(parentID TaskID, child *Task) *Task {
		child.ParentTaskID = parentID
		return child
	}
	blockedBy := func(child *Task, blockerIDs ...TaskID) *Task {
		child.BlockedBy = blockerIDs
		return child
	}

	cases := []struct {
		name   string
		tasks  []*Task
		filter ListTasksFilter
		want   []ListedTask
	}{
		{
			name: "lists every task newest first",
			tasks: []*Task{
				stored("oldest-todo", StatusTodo, "T-1", 0),
				stored("newest-draft", StatusDraft, "", 2),
				stored("middle-todo", StatusTodo, "T-2", 1),
			},
			want: []ListedTask{
				{Task: stored("newest-draft", StatusDraft, "", 2)},
				{Task: stored("middle-todo", StatusTodo, "T-2", 1)},
				{Task: stored("oldest-todo", StatusTodo, "T-1", 0)},
			},
		},
		{
			name: "lists the children under their parent, newest first",
			tasks: []*Task{
				childOf("parent", stored("older-child", StatusTodo, "", 1)),
				stored("parent", StatusTodo, "", 0),
				childOf("parent", stored("newer-child", StatusDone, "", 2)),
			},
			want: []ListedTask{
				{Task: stored("parent", StatusTodo, "", 0)},
				{Task: childOf("parent", stored("newer-child", StatusDone, "", 2)), IsUnderParent: true},
				{Task: childOf("parent", stored("older-child", StatusTodo, "", 1)), IsUnderParent: true},
			},
		},
		{
			name: "keeps parents and ungrouped tasks in one newest first order",
			tasks: []*Task{
				stored("old-parent", StatusTodo, "", 0),
				stored("ungrouped", StatusTodo, "", 1),
				stored("new-parent", StatusTodo, "", 2),
				childOf("old-parent", stored("old-child", StatusTodo, "", 5)),
				childOf("new-parent", stored("new-child", StatusTodo, "", 3)),
				stored("newest", StatusTodo, "", 4),
			},
			want: []ListedTask{
				{Task: stored("newest", StatusTodo, "", 4)},
				{Task: stored("new-parent", StatusTodo, "", 2)},
				{Task: childOf("new-parent", stored("new-child", StatusTodo, "", 3)), IsUnderParent: true},
				{Task: stored("ungrouped", StatusTodo, "", 1)},
				{Task: stored("old-parent", StatusTodo, "", 0)},
				{Task: childOf("old-parent", stored("old-child", StatusTodo, "", 5)), IsUnderParent: true},
			},
		},
		{
			name: "lists a task whose parent is gone among the others",
			tasks: []*Task{
				stored("ungrouped", StatusTodo, "", 0),
				childOf("gone", stored("orphan", StatusTodo, "", 1)),
			},
			want: []ListedTask{
				{Task: childOf("gone", stored("orphan", StatusTodo, "", 1))},
				{Task: stored("ungrouped", StatusTodo, "", 0)},
			},
		},
		{
			name: "lists every task of a chain deeper than one level",
			tasks: []*Task{
				stored("grandparent", StatusTodo, "", 0),
				childOf("grandparent", stored("parent", StatusTodo, "", 1)),
				childOf("parent", stored("child", StatusTodo, "", 2)),
			},
			want: []ListedTask{
				{Task: childOf("parent", stored("child", StatusTodo, "", 2))},
				{Task: stored("grandparent", StatusTodo, "", 0)},
				{Task: childOf("grandparent", stored("parent", StatusTodo, "", 1)), IsUnderParent: true},
			},
		},
		{
			name: "keeps the tasks in a status",
			tasks: []*Task{
				stored("oldest-todo", StatusTodo, "T-1", 0),
				stored("newest-draft", StatusDraft, "", 2),
				stored("middle-todo", StatusTodo, "T-2", 1),
			},
			filter: ListTasksFilter{Status: pointerTo(StatusTodo)},
			want: []ListedTask{
				{Task: stored("middle-todo", StatusTodo, "T-2", 1)},
				{Task: stored("oldest-todo", StatusTodo, "T-1", 0)},
			},
		},
		{
			name: "keeps the tasks of a ticket",
			tasks: []*Task{
				stored("oldest-todo", StatusTodo, "T-1", 0),
				stored("newest-draft", StatusDraft, "", 2),
			},
			filter: ListTasksFilter{TicketID: pointerTo("T-1")},
			want:   []ListedTask{{Task: stored("oldest-todo", StatusTodo, "T-1", 0)}},
		},
		{
			name: "an empty ticket keeps the tasks with no ticket",
			tasks: []*Task{
				stored("oldest-todo", StatusTodo, "T-1", 0),
				stored("newest-draft", StatusDraft, "", 2),
			},
			filter: ListTasksFilter{TicketID: pointerTo("")},
			want:   []ListedTask{{Task: stored("newest-draft", StatusDraft, "", 2)}},
		},
		{
			name: "combines the status and the ticket",
			tasks: []*Task{
				stored("oldest-todo", StatusTodo, "T-1", 0),
				stored("middle-todo", StatusTodo, "T-2", 1),
				stored("newest-draft", StatusDraft, "T-2", 2),
			},
			filter: ListTasksFilter{Status: pointerTo(StatusTodo), TicketID: pointerTo("T-2")},
			want:   []ListedTask{{Task: stored("middle-todo", StatusTodo, "T-2", 1)}},
		},
		{
			name:   "a status no task has keeps nothing",
			tasks:  []*Task{stored("oldest-todo", StatusTodo, "T-1", 0)},
			filter: ListTasksFilter{Status: pointerTo(StatusDone)},
			want:   nil,
		},
		{
			name: "a filtered listing is flat",
			tasks: []*Task{
				stored("parent", StatusTodo, "", 0),
				childOf("parent", stored("child", StatusTodo, "", 1)),
			},
			filter: ListTasksFilter{Status: pointerTo(StatusTodo)},
			want: []ListedTask{
				{Task: childOf("parent", stored("child", StatusTodo, "", 1))},
				{Task: stored("parent", StatusTodo, "", 0)},
			},
		},
		{
			name: "keeps a child whose parent the filter leaves out",
			tasks: []*Task{
				stored("parent", StatusInProgress, "", 0),
				childOf("parent", stored("child", StatusTodo, "", 1)),
			},
			filter: ListTasksFilter{Status: pointerTo(StatusTodo)},
			want:   []ListedTask{{Task: childOf("parent", stored("child", StatusTodo, "", 1))}},
		},
		{
			name: "keeps the tasks of a parent named by a prefix",
			tasks: []*Task{
				stored("parent", StatusTodo, "", 0),
				childOf("parent", stored("older-child", StatusTodo, "", 1)),
				childOf("parent", stored("newer-child", StatusDone, "", 2)),
				childOf("other", stored("other-child", StatusTodo, "", 3)),
				stored("other", StatusTodo, "", 4),
			},
			filter: ListTasksFilter{ParentID: pointerTo[TaskID]("par")},
			want: []ListedTask{
				{Task: childOf("parent", stored("newer-child", StatusDone, "", 2))},
				{Task: childOf("parent", stored("older-child", StatusTodo, "", 1))},
			},
		},
		{
			name: "combines the parent and the status",
			tasks: []*Task{
				stored("parent", StatusTodo, "", 0),
				childOf("parent", stored("older-child", StatusTodo, "", 1)),
				childOf("parent", stored("newer-child", StatusDone, "", 2)),
			},
			filter: ListTasksFilter{ParentID: pointerTo[TaskID]("parent"), Status: pointerTo(StatusTodo)},
			want:   []ListedTask{{Task: childOf("parent", stored("older-child", StatusTodo, "", 1))}},
		},
		{
			name: "a parent with no children keeps nothing",
			tasks: []*Task{
				stored("lonely", StatusTodo, "", 0),
				stored("other", StatusTodo, "", 1),
			},
			filter: ListTasksFilter{ParentID: pointerTo[TaskID]("lonely")},
			want:   nil,
		},
		{
			name: "an empty parent keeps the tasks that belong to no other task",
			tasks: []*Task{
				stored("parent", StatusTodo, "", 0),
				childOf("parent", stored("child", StatusTodo, "", 1)),
			},
			filter: ListTasksFilter{ParentID: pointerTo[TaskID]("")},
			want:   []ListedTask{{Task: stored("parent", StatusTodo, "", 0)}},
		},
		{
			name: "names the blockers that are not done",
			tasks: []*Task{
				stored("done-blocker", StatusDone, "", 0),
				stored("todo-blocker", StatusTodo, "", 1),
				stored("draft-blocker", StatusDraft, "", 2),
				blockedBy(stored("blocked-by-one", StatusTodo, "", 3), "done-blocker", "todo-blocker"),
				blockedBy(stored("blocked-by-several", StatusTodo, "", 4), "todo-blocker", "done-blocker", "draft-blocker"),
				blockedBy(stored("blocked-by-done", StatusTodo, "", 5), "done-blocker"),
			},
			want: []ListedTask{
				{Task: blockedBy(stored("blocked-by-done", StatusTodo, "", 5), "done-blocker")},
				{
					Task:    blockedBy(stored("blocked-by-several", StatusTodo, "", 4), "todo-blocker", "done-blocker", "draft-blocker"),
					Holding: []TaskID{"todo-blocker", "draft-blocker"},
				},
				{
					Task:    blockedBy(stored("blocked-by-one", StatusTodo, "", 3), "done-blocker", "todo-blocker"),
					Holding: []TaskID{"todo-blocker"},
				},
				{Task: stored("draft-blocker", StatusDraft, "", 2)},
				{Task: stored("todo-blocker", StatusTodo, "", 1)},
				{Task: stored("done-blocker", StatusDone, "", 0)},
			},
		},
		{
			name: "names a blocker the filter leaves out",
			tasks: []*Task{
				stored("draft-blocker", StatusDraft, "", 0),
				blockedBy(stored("blocked", StatusTodo, "", 1), "draft-blocker"),
			},
			filter: ListTasksFilter{Status: pointerTo(StatusTodo)},
			want: []ListedTask{
				{Task: blockedBy(stored("blocked", StatusTodo, "", 1), "draft-blocker"), Holding: []TaskID{"draft-blocker"}},
			},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			repo := &fakeTaskRepo{tasks: testCase.tasks}
			service := NewTaskService(repo, common.NewLogger(""))

			listed, err := service.ListTasks(testProjectSlug, testCase.filter, 1, 0)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(listed.Items, testCase.want) {
				t.Errorf("expected:\n%s\ngot:\n%s", describeListing(testCase.want), describeListing(listed.Items))
			}
		})
	}
}

func TestTaskService_ListTasks_RefusesAParentThatNamesNoTask(t *testing.T) {
	repo := &fakeTaskRepo{tasks: []*Task{{ID: "parent"}}}
	service := NewTaskService(repo, common.NewLogger(""))

	_, err := service.ListTasks(testProjectSlug, ListTasksFilter{ParentID: pointerTo[TaskID]("missing")}, 1, 0)

	if err == nil || !strings.Contains(err.Error(), `task "missing" not found`) {
		t.Fatalf("expected the parent to be not found, got %v", err)
	}
}

func TestTaskService_ListTasks_Pages(t *testing.T) {
	monday := time.Date(2026, 9, 21, 9, 0, 0, 0, time.UTC)
	stored := func(id TaskID, status TaskStatus, daysAfterMonday int, parentID TaskID) *Task {
		return &Task{ID: id, Status: status, ParentTaskID: parentID, CreatedAt: monday.AddDate(0, 0, daysAfterMonday)}
	}
	tasks := []*Task{
		stored("parent", StatusTodo, 0, ""),
		stored("first-child", StatusTodo, 1, "parent"),
		stored("second-child", StatusDone, 2, "parent"),
		stored("todo", StatusTodo, 3, ""),
		stored("done", StatusDone, 4, ""),
		stored("newest-todo", StatusTodo, 5, ""),
	}

	cases := []struct {
		name   string
		filter ListTasksFilter
		page   int
		size   int
		want   []ListedTask
		// wantTotalItems and wantTotalPages count the rows of every page.
		wantTotalItems int
		wantTotalPages int
		// wantErr is the whole refusal. A case without it expects a page.
		wantErr string
	}{
		{
			name: "a page that starts with children repeats their parent as a context row",
			page: 3,
			size: 2,
			want: []ListedTask{
				{Task: stored("parent", StatusTodo, 0, ""), IsContext: true},
				{Task: stored("second-child", StatusDone, 2, "parent"), IsUnderParent: true},
				{Task: stored("first-child", StatusTodo, 1, "parent"), IsUnderParent: true},
			},
			wantTotalItems: 6,
			wantTotalPages: 3,
		},
		{
			name: "a page that starts with the last child repeats its parent",
			page: 2,
			size: 5,
			want: []ListedTask{
				{Task: stored("parent", StatusTodo, 0, ""), IsContext: true},
				{Task: stored("first-child", StatusTodo, 1, "parent"), IsUnderParent: true},
			},
			wantTotalItems: 6,
			wantTotalPages: 2,
		},
		{
			name: "a page that starts with a top-level task gets no context row",
			page: 2,
			size: 3,
			want: []ListedTask{
				{Task: stored("parent", StatusTodo, 0, "")},
				{Task: stored("second-child", StatusDone, 2, "parent"), IsUnderParent: true},
				{Task: stored("first-child", StatusTodo, 1, "parent"), IsUnderParent: true},
			},
			wantTotalItems: 6,
			wantTotalPages: 2,
		},
		{
			name:   "a filtered listing counts only the rows it keeps and gets no context row",
			filter: ListTasksFilter{Status: pointerTo(StatusTodo)},
			page:   2,
			size:   2,
			want: []ListedTask{
				{Task: stored("first-child", StatusTodo, 1, "parent")},
				{Task: stored("parent", StatusTodo, 0, "")},
			},
			wantTotalItems: 4,
			wantTotalPages: 2,
		},
		{
			name:    "a page past the last one names the tasks",
			filter:  ListTasksFilter{Status: pointerTo(StatusDone)},
			page:    2,
			size:    2,
			wantErr: "page 2 does not exist, there is 1 page of tasks",
		},
		{
			name:    "a page below 1",
			page:    0,
			size:    2,
			wantErr: "page must be 1 or more, got 0",
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			service := NewTaskService(&fakeTaskRepo{tasks: tasks}, common.NewLogger(""))

			listed, err := service.ListTasks(testProjectSlug, testCase.filter, testCase.page, testCase.size)
			if testCase.wantErr != "" {
				if err == nil || err.Error() != testCase.wantErr {
					t.Fatalf("error = %v, want %q", err, testCase.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(listed.Items, testCase.want) {
				t.Errorf("expected:\n%s\ngot:\n%s", describeListing(testCase.want), describeListing(listed.Items))
			}
			if listed.TotalItems != testCase.wantTotalItems || listed.TotalPages != testCase.wantTotalPages {
				t.Errorf("%d items on %d pages, want %d items on %d pages",
					listed.TotalItems, listed.TotalPages, testCase.wantTotalItems, testCase.wantTotalPages)
			}
		})
	}
}

func describeListing(listed []ListedTask) string {
	var description string
	for _, entry := range listed {
		description += string(entry.Task.ID)
		if entry.IsContext {
			description += " as context"
		}
		if entry.IsUnderParent {
			description += " under parent"
		}
		for _, id := range entry.Holding {
			description += " held by " + string(id)
		}
		description += "\n"
	}
	return description
}
