package cmd

import (
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/IgorBolotnikov/DRUDGE/internal/common"
	"github.com/IgorBolotnikov/DRUDGE/internal/task"
	"github.com/IgorBolotnikov/DRUDGE/internal/theme"
)

func TestTaskListFlags(t *testing.T) {
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
			args: []string{"--status", "todo"},
			want: task.ListTasksFilter{Status: pointerTo(task.StatusTodo)},
		},
		{
			name:        "a status drudge does not know",
			args:        []string{"--status", "finished"},
			wantErrText: "finished",
		},
		{
			name: "a ticket",
			args: []string{"--ticket", "R-005"},
			want: task.ListTasksFilter{TicketID: pointerTo("R-005")},
		},
		{
			name: "a parent",
			args: []string{"--parent", "9c8d"},
			want: task.ListTasksFilter{ParentID: pointerTo[task.TaskID]("9c8d")},
		},
		{
			name: "an empty parent",
			args: []string{"--parent", ""},
			want: task.ListTasksFilter{ParentID: pointerTo[task.TaskID]("")},
		},
		{
			name: "every filter at once",
			args: []string{"--parent", "9c8d", "--status", "done", "--ticket", "R-005"},
			want: task.ListTasksFilter{
				Status:   pointerTo(task.StatusDone),
				TicketID: pointerTo("R-005"),
				ParentID: pointerTo[task.TaskID]("9c8d"),
			},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			flags := &taskListFlags{}
			parseTestFlags(t, flags.declare, testCase.args)

			filter, err := flags.filter()

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
			t.Setenv("HOME", t.TempDir())
			t.Setenv("NO_COLOR", "1")
			log := common.NewLogger("")

			out := captureOutput(func() { printTaskList(log, testCase.listed) })

			if out != testCase.want {
				t.Errorf("expected:\n%s\ngot:\n%s", testCase.want, out)
			}
		})
	}
}

func TestPrintTaskListColors(t *testing.T) {
	parent := &task.Task{ID: "9c8d7e6f-dbe9-4316-8aba-8a67a8f01f8f", Title: "Dependency tracking", Status: task.StatusInProgress}
	listed := []task.ListedTask{
		{Task: parent},
		{Task: &task.Task{ID: "2b3c4d5e-dbe9-4316-8aba-8a67a8f01f8f", Title: "Pick the next task", Status: task.StatusDone}, IsUnderParent: true},
		{Task: &task.Task{ID: "0a1b2c3d-dbe9-4316-8aba-8a67a8f01f8f", Title: "Write a draft", Status: task.StatusDraft}},
		{Task: &task.Task{ID: "1a2b3c4d-dbe9-4316-8aba-8a67a8f01f8f", Title: "Do the thing", Status: task.StatusTodo}},
		{Task: &task.Task{ID: "3c4d5e6f-dbe9-4316-8aba-8a67a8f01f8f", Title: "Break the build", Status: task.StatusFuckedUp}},
		{Task: &task.Task{ID: "4d5e6f7a-dbe9-4316-8aba-8a67a8f01f8f", Title: "Wait for a merge", Status: task.StatusUnmerged}},
		{Task: &task.Task{ID: "5e6f7a8b-dbe9-4316-8aba-8a67a8f01f8f", Title: "Edited by hand", Status: "finished"}},
	}
	const header = "Tasks (7):\n" +
		"  STATUS           ID        TITLE                                     BLOCKED BY  TICKET\n" +
		"  ---------------  --------  ----------------------------------------  ----------  ------\n"
	plain := header +
		"  in-progress      9c8d7e6f  Dependency tracking\n" +
		"    done           2b3c4d5e    Pick the next task\n" +
		"  draft            0a1b2c3d  Write a draft\n" +
		"  todo             1a2b3c4d  Do the thing\n" +
		"  fucked-up        3c4d5e6f  Break the build\n" +
		"  unmerged         4d5e6f7a  Wait for a merge\n" +
		"  finished         5e6f7a8b  Edited by hand\n"

	cases := []struct {
		name          string
		env           map[string]string
		themeFile     string
		want          func(palette *theme.Theme) string
		wantErrorText string
	}{
		{
			name: "forced color paints each status in its role",
			env:  map[string]string{"FORCE_COLOR": "1"},
			want: func(palette *theme.Theme) string {
				paint := func(role, text string) string { return palette.Color(role) + text + palette.Reset() }
				return header +
					"  " + paint(theme.RoleInfo, "in-progress") + "      9c8d7e6f  Dependency tracking\n" +
					"    " + paint(theme.RoleSuccess, "done") + "           2b3c4d5e    Pick the next task\n" +
					"  " + paint(theme.RoleMuted, "draft") + "            0a1b2c3d  Write a draft\n" +
					"  todo             1a2b3c4d  Do the thing\n" +
					"  " + paint(theme.RoleError, "fucked-up") + "        3c4d5e6f  Break the build\n" +
					"  " + paint(theme.RoleWarning, "unmerged") + "         4d5e6f7a  Wait for a merge\n" +
					"  finished         5e6f7a8b  Edited by hand\n"
			},
		},
		{
			name: "no color prints every status plain",
			env:  map[string]string{"NO_COLOR": "1", "FORCE_COLOR": "1"},
			want: func(*theme.Theme) string { return plain },
		},
		{
			name:          "a theme that fails to load prints every status plain",
			env:           map[string]string{"FORCE_COLOR": "1"},
			themeFile:     `{"theme": "no-such-theme"}`,
			want:          func(*theme.Theme) string { return plain },
			wantErrorText: "no-such-theme",
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)
			t.Setenv("NO_COLOR", "")
			for name, value := range testCase.env {
				t.Setenv(name, value)
			}
			if testCase.themeFile != "" {
				if err := os.WriteFile(filepath.Join(home, common.ThemeConfigName), []byte(testCase.themeFile), common.DefaultFilePerm); err != nil {
					t.Fatal(err)
				}
			}
			log := common.NewLogger("")

			var out string
			errOut := captureStderr(func() { out = captureOutput(func() { printTaskList(log, listed) }) })

			if want := testCase.want(theme.NewTheme(theme.DefaultTheme())); out != want {
				t.Errorf("expected:\n%q\ngot:\n%q", want, out)
			}
			if testCase.wantErrorText == "" && errOut != "" {
				t.Errorf("expected nothing on stderr, got %q", errOut)
			}
			if testCase.wantErrorText != "" && strings.Count(errOut, testCase.wantErrorText) != 1 {
				t.Errorf("expected one error naming %q on stderr, got %q", testCase.wantErrorText, errOut)
			}
		})
	}
}

func captureStderr(f func()) string {
	orig := os.Stderr
	reader, writer, _ := os.Pipe()
	os.Stderr = writer
	f()
	writer.Close()
	os.Stderr = orig
	out, _ := io.ReadAll(reader)
	return string(out)
}
