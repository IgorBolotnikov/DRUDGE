package cmd

import (
	"reflect"
	"strings"
	"testing"

	"github.com/IgorBolotnikov/DRUDGE/internal/common"
	"github.com/IgorBolotnikov/DRUDGE/internal/task"
)

func TestParseTaskListArgs(t *testing.T) {
	cases := []struct {
		name        string
		args        []string
		want        task.ListTasksFilter
		wantErrText string
	}{
		{
			name: "no flags filter nothing",
		},
		{
			name: "a status",
			args: []string{statusFlag, "todo"},
			want: task.ListTasksFilter{Status: pointerTo(task.StatusTodo)},
		},
		{
			name:        "a status drudge does not know",
			args:        []string{statusFlag, "finished"},
			wantErrText: "finished",
		},
		{
			name: "a ticket",
			args: []string{ticketFlag, "R-005"},
			want: task.ListTasksFilter{TicketID: pointerTo("R-005")},
		},
		{
			name: "a parent",
			args: []string{parentFlag, "9c8d"},
			want: task.ListTasksFilter{ParentID: pointerTo[task.TaskID]("9c8d")},
		},
		{
			name: "an empty parent",
			args: []string{parentFlag, ""},
			want: task.ListTasksFilter{ParentID: pointerTo[task.TaskID]("")},
		},
		{
			name: "every filter at once",
			args: []string{parentFlag, "9c8d", statusFlag, "done", ticketFlag, "R-005"},
			want: task.ListTasksFilter{
				Status:   pointerTo(task.StatusDone),
				TicketID: pointerTo("R-005"),
				ParentID: pointerTo[task.TaskID]("9c8d"),
			},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			filter, err := parseTaskListArgs(testCase.args)

			if testCase.wantErrText != "" {
				if err == nil || !strings.Contains(err.Error(), testCase.wantErrText) {
					t.Fatalf("expected an error naming %q, got %v", testCase.wantErrText, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(filter, testCase.want) {
				t.Errorf("expected %+v, got %+v", testCase.want, filter)
			}
		})
	}
}

func TestPrintTaskList(t *testing.T) {
	parent := &task.Task{ID: "9c8d7e6f-dbe9-4316-8aba-8a67a8f01f8f", Title: "Dependency tracking", Status: task.StatusTodo, TicketID: "R-005"}
	next := &task.Task{ID: "2b3c4d5e-dbe9-4316-8aba-8a67a8f01f8f", Title: "Pick the next task", Status: task.StatusTodo}
	storage := &task.Task{ID: "4f2a1b3c-dbe9-4316-8aba-8a67a8f01f8f", Title: "Store the blockers", Status: task.StatusTodo}
	cycle := &task.Task{ID: "7e6d5c4b-dbe9-4316-8aba-8a67a8f01f8f", Title: "Refuse a cycle", Status: task.StatusDone}
	workspaces := &task.Task{ID: "1a2b3c4d-dbe9-4316-8aba-8a67a8f01f8f", Title: "Drudger workspaces", Status: task.StatusInProgress, TicketID: "R-004"}

	cases := []struct {
		name   string
		listed []task.ListedTask
		want   string
	}{
		{
			name:   "no tasks",
			listed: nil,
			want:   "No tasks found\n",
		},
		{
			name:   "tasks that belong to no other task",
			listed: []task.ListedTask{{Task: parent}, {Task: workspaces}},
			want: "Tasks (2):\n" +
				"  STATUS           ID        TITLE                                     BLOCKED BY  TICKET\n" +
				"  ---------------  --------  ----------------------------------------  ----------  ------\n" +
				"  todo             9c8d7e6f  Dependency tracking                                   R-005\n" +
				"  in-progress      1a2b3c4d  Drudger workspaces                                    R-004\n",
		},
		{
			name: "children under their parent",
			listed: []task.ListedTask{
				{Task: parent},
				{Task: next, IsUnderParent: true, Holding: []task.TaskID{storage.ID}},
				{Task: storage, IsUnderParent: true},
				{Task: cycle, IsUnderParent: true},
				{Task: workspaces},
			},
			want: "Tasks (5):\n" +
				"  STATUS           ID        TITLE                                     BLOCKED BY  TICKET\n" +
				"  ---------------  --------  ----------------------------------------  ----------  ------\n" +
				"  todo             9c8d7e6f  Dependency tracking                                   R-005\n" +
				"    todo           2b3c4d5e    Pick the next task                      4f2a1b3c\n" +
				"    todo           4f2a1b3c    Store the blockers\n" +
				"    done           7e6d5c4b    Refuse a cycle\n" +
				"  in-progress      1a2b3c4d  Drudger workspaces                                    R-004\n",
		},
		{
			name: "a task held by several blockers",
			listed: []task.ListedTask{
				{Task: next, Holding: []task.TaskID{storage.ID, cycle.ID}},
				{Task: workspaces},
			},
			want: "Tasks (2):\n" +
				"  STATUS           ID        TITLE                                     BLOCKED BY          TICKET\n" +
				"  ---------------  --------  ----------------------------------------  ------------------  ------\n" +
				"  todo             2b3c4d5e  Pick the next task                        4f2a1b3c, 7e6d5c4b\n" +
				"  in-progress      1a2b3c4d  Drudger workspaces                                            R-004\n",
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			log := common.NewLogger("")

			out := captureOutput(func() { printTaskList(log, testCase.listed) })

			if out != testCase.want {
				t.Errorf("expected:\n%s\ngot:\n%s", testCase.want, out)
			}
		})
	}
}
