package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/IgorBolotnikov/DRUDGE/internal/adapters/persistence"
	"github.com/IgorBolotnikov/DRUDGE/internal/common"
	"github.com/IgorBolotnikov/DRUDGE/internal/config"
	"github.com/IgorBolotnikov/DRUDGE/internal/task"
)

func TestDescribeRemoval(t *testing.T) {
	removed := &task.Task{ID: "9c8d7e6f-dbe9-4316-8aba-8a67a8f01f8f", Title: "Add the migration"}
	wireRepository := &task.Task{ID: "4f2a1b3c-dbe9-4316-8aba-8a67a8f01f8f", Title: "Wire the repository"}
	addEndpoint := &task.Task{ID: "7e6d5c4b-dbe9-4316-8aba-8a67a8f01f8f", Title: "Add the endpoint"}
	pickNext := &task.Task{ID: "2b3c4d5e-dbe9-4316-8aba-8a67a8f01f8f", Title: "Pick the next task"}

	cases := []struct {
		name       string
		dependents []*task.Task
		children   []*task.Task
		want       []string
	}{
		{
			name: "a task with no links",
			want: []string{`task 9c8d7e6f-dbe9-4316-8aba-8a67a8f01f8f "Add the migration"`},
		},
		{
			name:       "dependents",
			dependents: []*task.Task{wireRepository, addEndpoint},
			want: []string{
				`task 9c8d7e6f-dbe9-4316-8aba-8a67a8f01f8f "Add the migration"`,
				"2 tasks are blocked by it and will be unblocked:",
				"  4f2a1b3c  Wire the repository",
				"  7e6d5c4b  Add the endpoint",
			},
		},
		{
			name:     "one child",
			children: []*task.Task{pickNext},
			want: []string{
				`task 9c8d7e6f-dbe9-4316-8aba-8a67a8f01f8f "Add the migration"`,
				"1 task belongs to it and will be ungrouped:",
				"  2b3c4d5e  Pick the next task",
			},
		},
		{
			name:       "one dependent and children",
			dependents: []*task.Task{wireRepository},
			children:   []*task.Task{addEndpoint, pickNext},
			want: []string{
				`task 9c8d7e6f-dbe9-4316-8aba-8a67a8f01f8f "Add the migration"`,
				"1 task is blocked by it and will be unblocked:",
				"  4f2a1b3c  Wire the repository",
				"2 tasks belong to it and will be ungrouped:",
				"  7e6d5c4b  Add the endpoint",
				"  2b3c4d5e  Pick the next task",
			},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			removal := task.Removal{Task: removed, Dependents: testCase.dependents, Children: testCase.children}

			got := describeRemoval(removal)
			if want := strings.Join(testCase.want, "\n"); got != want {
				t.Errorf("expected:\n%s\ngot:\n%s", want, got)
			}
		})
	}
}

func TestTaskRemove(t *testing.T) {
	monday := time.Date(2026, 9, 21, 9, 0, 0, 0, time.UTC)

	cases := []struct {
		name   string
		answer string
		// want renders stdout from the removed task.
		want        func(removed *task.Task) string
		wantRemoved bool
	}{
		{
			name:   "a task with a run directory, a dependent and a child",
			answer: "y\n",
			want: func(removed *task.Task) string {
				return deletionQuestion + "\n" +
					"Removing task " + task.ShortID(removed.ID) + "  Add the migration\n" +
					"  ✓ Removed its run directory\n" +
					"  · Branch drudge/add-the-migration stays, project demo records no repository ui\n" +
					"  ✓ Took it off the blockers of 1 task\n" +
					"  ✓ Ungrouped 1 task that belonged to it\n" +
					"✓ Removed task " + task.ShortID(removed.ID) + "  Add the migration\n"
			},
			wantRemoved: true,
		},
		{
			name:   "a declined prompt",
			answer: "n\n",
			want: func(removed *task.Task) string {
				return deletionQuestion + "· Left task " + task.ShortID(removed.ID) + "  Add the migration alone\n"
			},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			t.Setenv("NO_COLOR", "1")
			t.Chdir(t.TempDir())
			localConfig := &config.LocalConfig{ProjectSlug: "demo"}
			if err := localConfig.Save(); err != nil {
				t.Fatal(err)
			}
			repo := persistence.NewFileTaskRepository(localConfig.ProjectSlug)
			removed, err := repo.CreateTask(task.CreateTaskDto{Title: "Add the migration", Status: task.StatusTodo, ProjectSlug: localConfig.ProjectSlug, CreatedAt: monday})
			if err != nil {
				t.Fatal(err)
			}
			err = repo.UpdateTask(localConfig.ProjectSlug, removed.ID, func(stored *task.Task) error {
				stored.Landings = map[string]task.Landing{"ui": {Branch: "drudge/add-the-migration"}}
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			linked := []task.CreateTaskDto{
				{Title: "Wire the repository", BlockedBy: []task.TaskID{removed.ID}},
				{Title: "Add the endpoint", ParentTaskID: removed.ID},
			}
			for age, dto := range linked {
				dto.Status = task.StatusTodo
				dto.ProjectSlug = localConfig.ProjectSlug
				dto.CreatedAt = monday.AddDate(0, 0, age+1)
				if _, err := repo.CreateTask(dto); err != nil {
					t.Fatal(err)
				}
			}
			runDir := common.LocalRunDir(string(removed.ID))
			if err := os.MkdirAll(runDir, 0o755); err != nil {
				t.Fatal(err)
			}

			answerPath := filepath.Join(t.TempDir(), "answer")
			if err := os.WriteFile(answerPath, []byte(testCase.answer), 0o644); err != nil {
				t.Fatal(err)
			}
			answer, err := os.Open(answerPath)
			if err != nil {
				t.Fatal(err)
			}
			defer answer.Close()
			originalStdin := os.Stdin
			os.Stdin = answer
			defer func() { os.Stdin = originalStdin }()

			var output string
			stderr := captureStderr(func() {
				output = captureOutput(func() { err = NewRoot("v1.2.3").Execute([]string{TaskCmd.Name, "rm", string(removed.ID)}) })
			})

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if want := testCase.want(removed); output != want {
				t.Errorf("stdout:\n%q\nwant:\n%q", output, want)
			}
			if !strings.HasPrefix(stderr, "! This will permanently delete task "+string(removed.ID)) {
				t.Errorf("expected stderr to warn about the deletion, got %q", stderr)
			}
			_, lookupErr := repo.GetTask(localConfig.ProjectSlug, removed.ID)
			if isRemoved := lookupErr != nil; isRemoved != testCase.wantRemoved {
				t.Errorf("expected the task to be removed: %v, got %v", testCase.wantRemoved, isRemoved)
			}
			_, statErr := os.Stat(runDir)
			if isRunLeft := statErr == nil; isRunLeft == testCase.wantRemoved {
				t.Errorf("expected the run directory to be removed: %v, got %v", testCase.wantRemoved, !isRunLeft)
			}
		})
	}
}
