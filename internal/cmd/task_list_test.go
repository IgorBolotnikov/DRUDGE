package cmd

import (
	"encoding/json"
	"io"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/IgorBolotnikov/DRUDGE/internal/adapters/persistence"
	"github.com/IgorBolotnikov/DRUDGE/internal/common"
	"github.com/IgorBolotnikov/DRUDGE/internal/config"
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
			log := common.NewLogger("", common.Labels{})

			out := captureOutput(func() { printTaskList(log, onePage(testCase.listed)) })

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
				paint := func(role, text string) string { return palette.Paint(theme.Stdout, role, text) }
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
			name:      "the system theme paints each status in its SGR color",
			env:       map[string]string{"FORCE_COLOR": "1"},
			themeFile: `{"theme": "system"}`,
			want: func(*theme.Theme) string {
				paint := func(sgr, text string) string { return "\x1b[" + sgr + "m" + text + "\x1b[0m" }
				return header +
					"  " + paint("36", "in-progress") + "      9c8d7e6f  Dependency tracking\n" +
					"    " + paint("32", "done") + "           2b3c4d5e    Pick the next task\n" +
					"  " + paint("2", "draft") + "            0a1b2c3d  Write a draft\n" +
					"  todo             1a2b3c4d  Do the thing\n" +
					"  " + paint("31", "fucked-up") + "        3c4d5e6f  Break the build\n" +
					"  " + paint("33", "unmerged") + "         4d5e6f7a  Wait for a merge\n" +
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
				if err := os.MkdirAll(common.DrudgeDir(home), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(common.ThemeConfigPath(home), []byte(testCase.themeFile), common.DefaultFilePerm); err != nil {
					t.Fatal(err)
				}
			}
			log := common.NewLogger("", common.Labels{})

			var out string
			errOut := captureStderr(func() { out = captureOutput(func() { printTaskList(log, onePage(listed)) }) })

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

func TestPrintTaskListContextRow(t *testing.T) {
	parent := &task.Task{ID: "9c8d7e6f-dbe9-4316-8aba-8a67a8f01f8f", Title: "Dependency tracking", Status: task.StatusInProgress, TicketID: "R-005"}
	child := &task.Task{ID: "2b3c4d5e-dbe9-4316-8aba-8a67a8f01f8f", Title: "Pick the next task", Status: task.StatusDone}
	listed := common.Page[task.ListedTask]{
		Items: []task.ListedTask{
			{Task: parent, IsContext: true},
			{Task: child, IsUnderParent: true},
		},
		Number:     2,
		TotalPages: 2,
		TotalItems: 2,
	}
	const header = "Tasks (2):\n" +
		"  STATUS           ID        TITLE                                     BLOCKED BY  TICKET\n" +
		"  ---------------  --------  ----------------------------------------  ----------  ------\n"
	const contextRow = "in-progress      9c8d7e6f  Dependency tracking                                   R-005"

	cases := []struct {
		name string
		env  map[string]string
		want func(palette *theme.Theme) string
	}{
		{
			name: "forced color mutes the whole context row",
			env:  map[string]string{"FORCE_COLOR": "1"},
			want: func(palette *theme.Theme) string {
				paint := func(role, text string) string { return palette.Paint(theme.Stdout, role, text) }
				return header +
					"  " + paint(theme.RoleMuted, contextRow) + "\n" +
					"    " + paint(theme.RoleSuccess, "done") + "           2b3c4d5e    Pick the next task\n" +
					paint(theme.RoleMuted, "Page 2 of 2") + "\n"
			},
		},
		{
			name: "no color prints the context row plain",
			env:  map[string]string{"NO_COLOR": "1", "FORCE_COLOR": "1"},
			want: func(*theme.Theme) string {
				return header +
					"  " + contextRow + "\n" +
					"    done           2b3c4d5e    Pick the next task\n" +
					"Page 2 of 2\n"
			},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			t.Setenv("NO_COLOR", "")
			for name, value := range testCase.env {
				t.Setenv(name, value)
			}
			log := common.NewLogger("", common.Labels{})

			out := captureOutput(func() { printTaskList(log, listed) })

			if want := testCase.want(theme.NewTheme(theme.DefaultTheme())); out != want {
				t.Errorf("expected:\n%q\ngot:\n%q", want, out)
			}
		})
	}
}

func TestTaskList(t *testing.T) {
	const header = "  STATUS           ID        TITLE                                     BLOCKED BY  TICKET\n" +
		"  ---------------  --------  ----------------------------------------  ----------  ------\n"
	monday := time.Date(2026, 9, 21, 9, 0, 0, 0, time.UTC)
	threeTitles := []string{"Oldest task", "Middle task", "Newest task"}

	cases := []struct {
		name string
		// titles are the tasks to create, oldest first.
		titles []string
		// parents maps the index of a title to the index of its parent.
		parents map[int]int
		args    []string
		// localTask is the task section of the local config when set.
		localTask string
		// want renders the output from the short ids of the tasks.
		want    func(ids []string) string
		wantErr string
	}{
		{
			name:   "the first of two pages hints at the next one",
			titles: threeTitles,
			args:   []string{"--page-size", "2"},
			want: func(ids []string) string {
				return "Tasks (3):\n" + header +
					"  todo             " + ids[2] + "  Newest task\n" +
					"  todo             " + ids[1] + "  Middle task\n" +
					"Page 1 of 2, see the next one with --page 2\n"
			},
		},
		{
			name:   "the last of two pages prints a footer without a hint",
			titles: threeTitles,
			args:   []string{"-p", "2", "--page-size", "2"},
			want: func(ids []string) string {
				return "Tasks (3):\n" + header +
					"  todo             " + ids[0] + "  Oldest task\n" +
					"Page 2 of 2\n"
			},
		},
		{
			name:      "the local config sets the page size",
			titles:    threeTitles,
			args:      []string{"--page", "3"},
			localTask: `{"pageSize": 1}`,
			want: func(ids []string) string {
				return "Tasks (3):\n" + header +
					"  todo             " + ids[0] + "  Oldest task\n" +
					"Page 3 of 3\n"
			},
		},
		{
			name:      "a page size of 0 on the command line turns paging off",
			titles:    threeTitles[:2],
			args:      []string{"--page-size", "0"},
			localTask: `{"pageSize": 1}`,
			want: func(ids []string) string {
				return "Tasks (2):\n" + header +
					"  todo             " + ids[1] + "  Middle task\n" +
					"  todo             " + ids[0] + "  Oldest task\n"
			},
		},
		{
			name:    "a page that starts with a child repeats its parent without counting it",
			titles:  []string{"Parent task", "Child task", "Newest task"},
			parents: map[int]int{1: 0},
			args:    []string{"--page", "2", "--page-size", "2"},
			want: func(ids []string) string {
				return "Tasks (3):\n" + header +
					"  todo             " + ids[0] + "  Parent task\n" +
					"    todo           " + ids[1] + "    Child task\n" +
					"Page 2 of 2\n"
			},
		},
		{
			name: "no tasks",
			want: func([]string) string { return "No tasks found\n" },
		},
		{
			name:    "a page past the last one",
			titles:  threeTitles,
			args:    []string{"--page", "3", "--page-size", "2"},
			wantErr: "page 3 does not exist, there are 2 pages of tasks",
		},
		{
			name:    "a negative page size",
			args:    []string{"--page-size", "-1"},
			wantErr: "--page-size must be 0 or more, got -1",
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			t.Setenv("NO_COLOR", "1")
			t.Chdir(t.TempDir())
			localConfig := &config.LocalConfig{ProjectSlug: "demo"}
			if testCase.localTask != "" {
				if err := json.Unmarshal([]byte(testCase.localTask), &localConfig.Task); err != nil {
					t.Fatal(err)
				}
			}
			if err := localConfig.Save(); err != nil {
				t.Fatal(err)
			}
			repo := persistence.NewFileTaskRepository(localConfig.ProjectSlug)
			var ids []string
			var fullIDs []task.TaskID
			for age, title := range testCase.titles {
				var parentID task.TaskID
				if parent, ok := testCase.parents[age]; ok {
					parentID = fullIDs[parent]
				}
				created, err := repo.CreateTask(task.CreateTaskDto{
					Title:        title,
					Status:       task.StatusTodo,
					ProjectSlug:  localConfig.ProjectSlug,
					ParentTaskID: parentID,
					CreatedAt:    monday.AddDate(0, 0, age),
				})
				if err != nil {
					t.Fatal(err)
				}
				ids = append(ids, task.ShortID(created.ID))
				fullIDs = append(fullIDs, created.ID)
			}

			var err error
			args := append([]string{TaskCmd.Name, "list"}, testCase.args...)
			output := captureOutput(func() { err = NewRoot("v1.2.3").Execute(args) })

			if testCase.wantErr != "" {
				if err == nil || err.Error() != testCase.wantErr {
					t.Fatalf("error = %v, want %q", err, testCase.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if want := testCase.want(ids); output != want {
				t.Errorf("expected:\n%q\ngot:\n%q", want, output)
			}
		})
	}
}

// onePage puts every row of a listing on a single page.
func onePage(listed []task.ListedTask) common.Page[task.ListedTask] {
	return common.Page[task.ListedTask]{Items: listed, Number: 1, TotalPages: 1, TotalItems: len(listed)}
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
