package report

import (
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/IgorBolotnikov/DRUDGE/internal/drudger"
	"github.com/IgorBolotnikov/DRUDGE/internal/project"
	"github.com/IgorBolotnikov/DRUDGE/internal/release"
	"github.com/IgorBolotnikov/DRUDGE/internal/task"
)

func plainTaskName(named *task.Task) string {
	return task.ShortID(named.ID) + "  " + named.Title
}

func TestLines(t *testing.T) {
	sampleTask := &task.Task{ID: "006684e3-dbe9-4316-8aba-8a67a8f01f8f", Title: "Fix login", Status: task.StatusTodo}
	doneTask := &task.Task{ID: sampleTask.ID, Title: sampleTask.Title, Status: task.StatusDone}
	fuckedUpTask := &task.Task{ID: sampleTask.ID, Title: sampleTask.Title, Status: task.StatusFuckedUp}
	refusedTask := &task.Task{ID: sampleTask.ID, Title: sampleTask.Title, Status: task.StatusTodo, VendorErrorClass: task.VendorErrorRateLimit}
	dependent := &task.Task{ID: "4f2a1b3c-dbe9-4316-8aba-8a67a8f01f8f", Title: "Wire the repository"}
	otherDependent := &task.Task{ID: "7e6d5c4b-dbe9-4316-8aba-8a67a8f01f8f", Title: "Add the endpoint"}

	cases := []struct {
		name  string
		event any
		want  []Line
	}{
		{
			name:  "a task created",
			event: task.TaskCreated{Task: sampleTask},
			want:  []Line{{Kind: KindLine, Level: LevelSuccess, Text: "Created task 006684e3  Fix login"}},
		},
		{
			name:  "a declined removal",
			event: task.TaskRemovalDeclined{Task: sampleTask},
			want:  []Line{{Kind: KindLine, Level: LevelSkipped, Text: "Left task 006684e3  Fix login alone"}},
		},
		{
			name:  "a removal started",
			event: task.TaskRemovalStarted{Task: sampleTask},
			want:  []Line{{Kind: KindHeader, Level: LevelInfo, Text: "Removing task 006684e3  Fix login"}},
		},
		{
			name:  "a task removed",
			event: task.TaskRemoved{Task: sampleTask},
			want:  []Line{{Kind: KindResult, Level: LevelSuccess, Text: "Removed task 006684e3  Fix login"}},
		},
		{
			name:  "an unlink held by another command",
			event: task.TaskUnlinkFailed{RemovedID: sampleTask.ID, LinkedID: dependent.ID, IsHeld: true},
			want: []Line{{
				Kind:  KindLine,
				Level: LevelWarning,
				Text:  "Task 006684e3-dbe9-4316-8aba-8a67a8f01f8f is removed, but another drudge command is working on task 4f2a1b3c-dbe9-4316-8aba-8a67a8f01f8f, which still names it",
			}},
		},
		{
			name:  "an import skipped as in progress",
			event: task.TaskImportSkipped{Task: sampleTask, Reason: task.SkipReasonInProgress},
			want:  []Line{{Kind: KindLine, Level: LevelSkipped, Text: "Skipped task 006684e3  Fix login, it is in progress here"}},
		},
		{
			name:  "an import that changed nothing",
			event: task.TasksImported{ProjectSlug: "demo", SkippedAsOlder: 2},
			want: []Line{{
				Kind:  KindResult,
				Level: LevelSkipped,
				Text:  "Changed no tasks in project demo, added 0, updated 0, skipped 2 as older, 0 as in progress and 0 as bad",
			}},
		},
		{
			name:  "an import",
			event: task.TasksImported{ProjectSlug: "demo", Added: 1},
			want: []Line{{
				Kind:  KindResult,
				Level: LevelSuccess,
				Text:  "Imported into project demo, added 1, updated 0, skipped 0 as older, 0 as in progress and 0 as bad",
			}},
		},
		{
			name:  "a project created shows nothing",
			event: project.ProjectCreated{},
		},
		{
			name:  "a download started",
			event: release.DownloadStarted{ArchiveName: "drg_linux_amd64.tar.gz", Version: "v1.2.0"},
			want:  []Line{{Kind: KindLine, Level: LevelStep, Text: "Downloading drg_linux_amd64.tar.gz (v1.2.0)"}},
		},
		{
			name:  "a Drudger claimed",
			event: drudger.DrudgerClaimed{Task: sampleTask, Slot: 2, Sandbox: "drudge-demo-2"},
			want:  []Line{{Kind: KindHeader, Level: LevelInfo, Text: "Task 006684e3  Fix login → Drudger 2 (drudge-demo-2)"}},
		},
		{
			name:  "a fetch failed",
			event: drudger.BaseFetchFailed{Repository: "api", Branch: "main", Err: errors.New("exit status 128")},
			want:  []Line{{Kind: KindLine, Level: LevelWarning, Text: "Could not fetch main of api: exit status 128"}},
		},
		{
			name:  "a stale base with no commit",
			event: drudger.StaleBaseUsed{Repository: "api", Ref: "origin/main"},
			want:  []Line{{Kind: KindLine, Level: LevelWarning, Text: "Work on api is cut from origin/main"}},
		},
		{
			name:  "sbx output",
			event: drudger.SbxOutput{Binary: "sbx", Line: "pulling image"},
			want:  []Line{{Kind: KindDetail, Level: LevelInfo, Text: "sbx: pulling image"}},
		},
		{
			name:  "an agent launch started",
			event: drudger.AgentLaunchStarted{GracePeriod: 30 * time.Second},
			want:  []Line{{Kind: KindLine, Level: LevelStep, Text: "Starting the agent, waiting up to 30s for its first output"}},
		},
		{
			name:  "an agent launched",
			event: drudger.AgentLaunched{Task: sampleTask, Sandbox: "drudge-demo-2", Branch: "drudge/0066-fix-login", RunDir: ".drudge/runs/006684e3"},
			want: []Line{
				{Kind: KindResult, Level: LevelSuccess, Text: "Drudger drudge-demo-2 is working on task 006684e3  Fix login"},
				{Kind: KindField, Level: LevelInfo, Label: "Branch", Text: "drudge/0066-fix-login"},
				{Kind: KindField, Level: LevelInfo, Label: "Run dir", Text: ".drudge/runs/006684e3"},
			},
		},
		{
			name: "a dry run",
			event: drudger.RunDescribed{
				Task:          sampleTask,
				Slot:          3,
				Sandbox:       "drudge-demo-3",
				PromptSource:  "~/.drudge/prompt.md",
				Prompt:        "Fix login\n",
				UnsetEnvNames: []string{"NPM_TOKEN"},
				Copies:        []drudger.LocalCopy{{Repository: "api", Path: ".env"}},
				Commands:      [][]string{{"sbx", "ls", "--json"}},
			},
			want: []Line{
				{Kind: KindHeader, Level: LevelInfo, Text: "Dry run of task 006684e3  Fix login on Drudger 3 (drudge-demo-3)"},
				{Kind: KindField, Level: LevelInfo, Label: "Prompt from", Text: "~/.drudge/prompt.md"},
				{Kind: KindLine, Level: LevelWarning, Text: "NPM_TOKEN is not set in this environment, a launch stops on it"},
				{Kind: KindBlock, Level: LevelInfo, Text: "Fix login\n"},
				{Kind: KindField, Level: LevelInfo, Label: "Copies"},
				{Kind: KindBlock, Level: LevelInfo, Text: "api  .env"},
				{Kind: KindField, Level: LevelInfo, Label: "Commands"},
				{Kind: KindBlock, Level: LevelInfo, Text: `"sbx" "ls" "--json"`},
				{Kind: KindResult, Level: LevelSkipped, Text: "Nothing ran, it was a dry run"},
			},
		},
		{
			name:  "a Session that got shit done",
			event: drudger.SessionRecorded{Task: doneTask, Status: drudger.StatusGotShitDone},
			want:  []Line{{Kind: KindResult, Level: LevelSuccess, Text: "Task 006684e3  Fix login got shit done, it is done"}},
		},
		{
			name:  "a Session that needs babysitting",
			event: drudger.SessionRecorded{Task: sampleTask, Status: drudger.StatusNeedsBabysitting},
			want:  []Line{{Kind: KindResult, Level: LevelWarning, Text: "Task 006684e3  Fix login needs babysitting, it is todo"}},
		},
		{
			name:  "a Session that fucked up",
			event: drudger.SessionRecorded{Task: fuckedUpTask, Status: drudger.StatusFuckedUp},
			want:  []Line{{Kind: KindResult, Level: LevelError, Text: "Task 006684e3  Fix login fucked up, it is fucked-up"}},
		},
		{
			name:  "dependents unblocked",
			event: drudger.DependentsUnblocked{Tasks: []*task.Task{dependent, otherDependent}},
			want: []Line{
				{Kind: KindField, Level: LevelInfo, Label: "Unblocked", Text: "4f2a1b3c  Wire the repository"},
				{Kind: KindField, Level: LevelInfo, Text: "7e6d5c4b  Add the endpoint"},
			},
		},
		{
			name:  "a run refused",
			event: drudger.RunRefused{Task: refusedTask},
			want: []Line{
				{Kind: KindResult, Level: LevelWarning, Text: "The vendor refused task 006684e3  Fix login (rate limit), it is back in todo"},
				{Kind: KindField, Level: LevelInfo, Label: "Advice", Text: "Run the task again once the vendor lets you through."},
			},
		},
		{
			name:  "some pull requests retried",
			event: drudger.PullRequestsRetried{Task: sampleTask, Tried: 2, Opened: 1},
			want:  []Line{{Kind: KindResult, Level: LevelWarning, Text: "Opened 1 of 2 pull requests of task 006684e3  Fix login"}},
		},
		{
			name:  "a task killed",
			event: drudger.TaskKilled{Task: fuckedUpTask},
			want:  []Line{{Kind: KindLine, Level: LevelError, Text: "Task 006684e3  Fix login is fucked-up, its agent was killed with the Drudger"}},
		},
		{
			name:  "a branch cleanup that could not delete",
			event: drudger.BranchCleanupFailed{Step: drudger.BranchDeleteStep, Repository: "api", Branch: "drudge/0066", Err: errors.New("locked")},
			want:  []Line{{Kind: KindLine, Level: LevelWarning, Text: "Could not delete branch drudge/0066 of repository api, it stays: locked"}},
		},
		{
			name:  "an event no interface shows",
			event: struct{}{},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			got := Lines(testCase.event, plainTaskName)
			if !slices.Equal(got, testCase.want) {
				t.Errorf("expected:\n%+v\ngot:\n%+v", testCase.want, got)
			}
		})
	}
}

func TestRemoval(t *testing.T) {
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

			got := Removal(removal)
			if want := strings.Join(testCase.want, "\n"); got != want {
				t.Errorf("expected:\n%s\ngot:\n%s", want, got)
			}
		})
	}
}
