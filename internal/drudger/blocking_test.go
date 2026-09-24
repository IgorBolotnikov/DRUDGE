package drudger

import (
	"errors"
	"io/fs"
	"maps"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/IgorBolotnikov/DRUDGE/internal/common"
	"github.com/IgorBolotnikov/DRUDGE/internal/config"
	"github.com/IgorBolotnikov/DRUDGE/internal/task"
)

// blockerLaunch is one of the ways a task is handed to an agent.
type blockerLaunch struct {
	name     string
	status   task.TaskStatus
	isRerun  bool
	isDryRun bool
}

var blockerLaunches = []blockerLaunch{
	{name: "run", status: task.StatusTodo},
	{name: "dry run", status: task.StatusTodo, isDryRun: true},
	{name: "rerun", status: task.StatusFuckedUp, isRerun: true},
	{name: "dry rerun", status: task.StatusFuckedUp, isRerun: true, isDryRun: true},
}

func (launch blockerLaunch) start(service *testService, taskID task.TaskID) error {
	if launch.isRerun {
		return service.RerunTask(testProjectSlug, taskID, launch.isDryRun)
	}
	return service.RunTask(testProjectSlug, taskID, launch.isDryRun)
}

func blockerTask(id task.TaskID, status task.TaskStatus, title string) *task.Task {
	return &task.Task{ID: id, Title: title, Status: status, ProjectSlug: testProjectSlug}
}

func TestDrudgerService_RefusesATaskWithAnUnfinishedBlocker(t *testing.T) {
	cases := []struct {
		name     string
		blockers []*task.Task
		// missingIDs are blocker ids that name no stored task.
		missingIDs []task.TaskID
		// wantLines are the lines the refusal lists, and none means the task runs.
		wantLines []string
	}{
		{
			name: "runs a task with no blockers",
		},
		{
			name:     "runs a task whose blocker is done",
			blockers: []*task.Task{blockerTask("9c8d7e6f-0001", task.StatusDone, "Add the migration")},
		},
		{
			name:      "refuses a draft blocker",
			blockers:  []*task.Task{blockerTask("9c8d7e6f-0001", task.StatusDraft, "Add the migration")},
			wantLines: []string{"  9c8d7e6f  draft        Add the migration"},
		},
		{
			name:      "refuses a todo blocker",
			blockers:  []*task.Task{blockerTask("9c8d7e6f-0001", task.StatusTodo, "Add the migration")},
			wantLines: []string{"  9c8d7e6f  todo         Add the migration"},
		},
		{
			name:      "refuses an in-progress blocker",
			blockers:  []*task.Task{blockerTask("9c8d7e6f-0001", task.StatusInProgress, "Add the migration")},
			wantLines: []string{"  9c8d7e6f  in-progress  Add the migration"},
		},
		{
			name:      "refuses a fucked-up blocker",
			blockers:  []*task.Task{blockerTask("9c8d7e6f-0001", task.StatusFuckedUp, "Add the migration")},
			wantLines: []string{"  9c8d7e6f  fucked-up    Add the migration"},
		},
		{
			name: "names every unfinished blocker and skips the done ones",
			blockers: []*task.Task{
				blockerTask("9c8d7e6f-0001", task.StatusInProgress, "Add the migration"),
				blockerTask("7e6d5c4b-0002", task.StatusDone, "Refuse a cycle"),
				blockerTask("1a2b3c4d-0003", task.StatusFuckedUp, "Wire the repository"),
			},
			wantLines: []string{
				"  9c8d7e6f  in-progress  Add the migration",
				"  1a2b3c4d  fucked-up    Wire the repository",
			},
		},
		{
			name:       "refuses a blocker id naming no task",
			missingIDs: []task.TaskID{"4f2a1b3c-0004"},
			wantLines:  []string{"  4f2a1b3c  no such task"},
		},
	}

	for _, testCase := range cases {
		for _, launch := range blockerLaunches {
			t.Run(testCase.name+" on a "+launch.name, func(t *testing.T) {
				projectDir := setupProjectDir(t)
				dependent := todoTask()
				dependent.Status = launch.status
				for _, blocker := range testCase.blockers {
					dependent.BlockedBy = append(dependent.BlockedBy, blocker.ID)
				}
				dependent.BlockedBy = append(dependent.BlockedBy, testCase.missingIDs...)

				commands := &fakeCommandRunner{projectDir: projectDir, outputs: []string{sandboxListingWith(testSandbox)}}
				stored := append([]*task.Task{dependent}, testCase.blockers...)
				service := newTestServiceWith(&config.LocalConfig{ProjectSlug: testProjectSlug}, config.DefaultConfig(), commands, stored...)

				var err error
				captureOutput(func() { err = launch.start(service, dependent.ID) })

				if len(testCase.wantLines) == 0 {
					if err != nil {
						t.Fatalf("unexpected error: %v", err)
					}
					if !launch.isDryRun && dependent.Status != task.StatusInProgress {
						t.Errorf("expected status %q, got %q", task.StatusInProgress, dependent.Status)
					}
					return
				}

				want := "task " + task.ShortID(dependent.ID) + " is blocked by:\n" + strings.Join(testCase.wantLines, "\n")
				if err == nil {
					t.Fatalf("expected the launch to be refused with %q", want)
				}
				if err.Error() != want {
					t.Errorf("expected the error\n%s\ngot\n%s", want, err)
				}
				if len(commands.calls) != 0 {
					t.Errorf("expected a refused launch to run nothing, got %v", commands.subcommands())
				}
				if dependent.Status != launch.status || !dependent.StartedAt.IsZero() {
					t.Errorf("expected the task record to be left alone, got status %q", dependent.Status)
				}
				if _, err := os.Stat(common.RunDir(projectDir, string(dependent.ID))); !errors.Is(err, fs.ErrNotExist) {
					t.Errorf("expected a refused launch to write no run directory, got %v", err)
				}
			})
		}
	}
}

// landedBlocker is a done task whose run left work in each repository named.
// The work in a repository sits at the commit landedHead names.
func landedBlocker(id task.TaskID, title string, repositories ...string) *task.Task {
	blocker := blockerTask(id, task.StatusDone, title)
	for _, repository := range repositories {
		blocker.RecordLanding(repository, task.Landing{Branch: landedBranch(id), Base: testBaseSHA, Head: landedHead(id, repository)})
	}
	return blocker
}

func landedBranch(id task.TaskID) string {
	return "task/" + task.ShortID(id)
}

func landedHead(id task.TaskID, repository string) string {
	return "head-" + task.ShortID(id) + "-" + repository
}

func TestDrudgerService_RefusesATaskWithUnmergedBlockerWork(t *testing.T) {
	cases := []struct {
		name string
		// repositories are the repositories of the project.
		repositories []string
		blockers     []*task.Task
		// outstanding is how many commits a landing holds that its base does
		// not, keyed by the commit landedHead names. A landing with no entry
		// has reached its base.
		outstanding map[string]int
		hasNoRemote bool
		fetchErr    error
		// wantLines are the lines the refusal lists, and none means the task runs.
		wantLines []string
		// wantFetched are the repositories the gate fetched before it decided.
		wantFetched []string
		// wantWarning is what the gate says on stderr.
		wantWarning string
	}{
		{
			name:         "runs a task whose done blocker left no work",
			repositories: []string{"drudge"},
			blockers:     []*task.Task{landedBlocker("9c8d7e6f-0001", "Add the migration")},
		},
		{
			name:         "runs a task whose blocker work has reached the base",
			repositories: []string{"drudge"},
			blockers:     []*task.Task{landedBlocker("9c8d7e6f-0001", "Add the migration", "drudge")},
		},
		{
			name:         "refuses work still on its branch",
			repositories: []string{"drudge"},
			blockers:     []*task.Task{landedBlocker("9c8d7e6f-0001", "Add the migration", "drudge")},
			outstanding:  map[string]int{landedHead("9c8d7e6f-0001", "drudge"): 3},
			wantLines: []string{
				"  9c8d7e6f  Add the migration",
				"    drudge  task/9c8d7e6f  3 commits not in origin/main",
			},
			wantFetched: []string{"drudge"},
		},
		{
			name:         "names the two repositories of three holding work",
			repositories: []string{"drudge", "drudge-web", "docs"},
			blockers:     []*task.Task{landedBlocker("9c8d7e6f-0001", "Add the migration", "drudge", "drudge-web", "docs")},
			outstanding: map[string]int{
				landedHead("9c8d7e6f-0001", "drudge"):     3,
				landedHead("9c8d7e6f-0001", "drudge-web"): 1,
			},
			wantLines: []string{
				"  9c8d7e6f  Add the migration",
				"    drudge      task/9c8d7e6f  3 commits not in origin/main",
				"    drudge-web  task/9c8d7e6f  1 commit not in origin/main",
			},
			wantFetched: []string{"drudge", "drudge-web", "docs"},
		},
		{
			name:         "names both blockers with work out",
			repositories: []string{"drudge"},
			blockers: []*task.Task{
				landedBlocker("9c8d7e6f-0001", "Add the migration", "drudge"),
				landedBlocker("7e6d5c4b-0002", "Refuse a cycle", "drudge"),
			},
			outstanding: map[string]int{
				landedHead("9c8d7e6f-0001", "drudge"): 3,
				landedHead("7e6d5c4b-0002", "drudge"): 2,
			},
			wantLines: []string{
				"  9c8d7e6f  Add the migration",
				"    drudge  task/9c8d7e6f  3 commits not in origin/main",
				"  7e6d5c4b  Refuse a cycle",
				"    drudge  task/7e6d5c4b  2 commits not in origin/main",
			},
			wantFetched: []string{"drudge"},
		},
		{
			name:         "ignores work in a repository the project no longer has",
			repositories: []string{"drudge"},
			blockers:     []*task.Task{landedBlocker("9c8d7e6f-0001", "Add the migration", "drudge", "gone")},
			outstanding:  map[string]int{landedHead("9c8d7e6f-0001", "gone"): 3},
		},
		{
			name:         "checks a repository with no remote against its local branch",
			repositories: []string{"drudge"},
			blockers:     []*task.Task{landedBlocker("9c8d7e6f-0001", "Add the migration", "drudge")},
			outstanding:  map[string]int{landedHead("9c8d7e6f-0001", "drudge"): 1},
			hasNoRemote:  true,
			wantLines: []string{
				"  9c8d7e6f  Add the migration",
				"    drudge  task/9c8d7e6f  1 commit not in main",
			},
		},
		{
			name:         "decides on what is on disk when the fetch fails",
			repositories: []string{"drudge"},
			blockers:     []*task.Task{landedBlocker("9c8d7e6f-0001", "Add the migration", "drudge")},
			outstanding:  map[string]int{landedHead("9c8d7e6f-0001", "drudge"): 3},
			fetchErr:     errors.New("network is unreachable"),
			wantLines: []string{
				"  9c8d7e6f  Add the migration",
				"    drudge  task/9c8d7e6f  3 commits not in origin/main",
			},
			wantFetched: []string{"drudge"},
			wantWarning: "checked against origin/main as it is on disk",
		},
	}

	for _, testCase := range cases {
		for _, launch := range blockerLaunches {
			t.Run(testCase.name+" on a "+launch.name, func(t *testing.T) {
				projectDir := setupProjectDir(t)
				dependent := todoTask()
				dependent.Status = launch.status
				for _, blocker := range testCase.blockers {
					dependent.BlockedBy = append(dependent.BlockedBy, blocker.ID)
				}

				// No sandbox exists yet, so a launch creates one over whatever
				// repositories the case names.
				commands := &fakeCommandRunner{projectDir: projectDir, outputs: []string{sandboxListingOf()}}
				stored := append([]*task.Task{dependent}, testCase.blockers...)
				service := newTestServiceWith(localConfigWith(testCase.repositories...), config.DefaultConfig(), commands, stored...)
				service.git.hasNoRemote = testCase.hasNoRemote
				service.git.fetchErr = testCase.fetchErr
				for head, count := range testCase.outstanding {
					service.git.commitsOn(head, count)
				}

				var err error
				warnings := captureErrors(func() {
					captureOutput(func() { err = launch.start(service, dependent.ID) })
				})

				if !strings.Contains(warnings, testCase.wantWarning) {
					t.Errorf("expected stderr to hold %q, got %q", testCase.wantWarning, warnings)
				}

				if len(testCase.wantLines) == 0 {
					if err != nil {
						t.Fatalf("unexpected error: %v", err)
					}
					if !launch.isDryRun && dependent.Status != task.StatusInProgress {
						t.Errorf("expected status %q, got %q", task.StatusInProgress, dependent.Status)
					}
					return
				}

				want := "task " + task.ShortID(dependent.ID) + " cannot run, work it depends on is not merged:\n" + strings.Join(testCase.wantLines, "\n")
				if err == nil {
					t.Fatalf("expected the launch to be refused with %q", want)
				}
				if err.Error() != want {
					t.Errorf("expected the error\n%s\ngot\n%s", want, err)
				}
				if wantFetched := pathsIn(projectDir, testCase.wantFetched...); !slices.Equal(service.git.fetched, wantFetched) {
					t.Errorf("expected the repositories %v to be fetched, got %v", wantFetched, service.git.fetched)
				}
				if len(commands.calls) != 0 {
					t.Errorf("expected a refused launch to run nothing, got %v", commands.subcommands())
				}
				if dependent.Status != launch.status || !dependent.StartedAt.IsZero() {
					t.Errorf("expected the task record to be left alone, got status %q", dependent.Status)
				}
				if _, err := os.Stat(common.RunDir(projectDir, string(dependent.ID))); !errors.Is(err, fs.ErrNotExist) {
					t.Errorf("expected a refused launch to write no run directory, got %v", err)
				}
			})
		}
	}
}

func TestDrudgerService_UnmergedWork(t *testing.T) {
	cases := []struct {
		name     string
		blockers []*task.Task
		// missingIDs are blocker ids that name no stored task.
		missingIDs  []task.TaskID
		outstanding map[string]int
		want        map[task.TaskID][]UnmergedWork
	}{
		{
			name: "reports nothing for no blockers",
		},
		{
			name:        "reports nothing for a blocker that is not done",
			blockers:    []*task.Task{blockerTask("9c8d7e6f-0001", task.StatusInProgress, "Add the migration")},
			missingIDs:  []task.TaskID{"4f2a1b3c-0004"},
			outstanding: map[string]int{landedHead("9c8d7e6f-0001", "drudge"): 3},
		},
		{
			name:     "reports nothing for work that reached the base",
			blockers: []*task.Task{landedBlocker("9c8d7e6f-0001", "Add the migration", "drudge")},
		},
		{
			name: "reports the work of a done blocker still on its branch",
			blockers: []*task.Task{
				landedBlocker("9c8d7e6f-0001", "Add the migration", "drudge"),
				landedBlocker("7e6d5c4b-0002", "Refuse a cycle", "drudge"),
			},
			outstanding: map[string]int{landedHead("9c8d7e6f-0001", "drudge"): 3},
			want: map[task.TaskID][]UnmergedWork{
				"9c8d7e6f-0001": {{Repository: "drudge", Branch: "task/9c8d7e6f", BaseRef: "origin/main", Commits: 3}},
			},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			setupProjectDir(t)
			dependent := todoTask()
			for _, blocker := range testCase.blockers {
				dependent.BlockedBy = append(dependent.BlockedBy, blocker.ID)
			}
			dependent.BlockedBy = append(dependent.BlockedBy, testCase.missingIDs...)

			stored := append([]*task.Task{dependent}, testCase.blockers...)
			service := newTestServiceWith(localConfigWith("drudge"), config.DefaultConfig(), &fakeCommandRunner{}, stored...)
			for head, count := range testCase.outstanding {
				service.git.commitsOn(head, count)
			}

			blockers, err := service.tasks.Blockers(testProjectSlug, dependent)
			if err != nil {
				t.Fatalf("Blockers: %v", err)
			}

			var got map[task.TaskID][]UnmergedWork
			captureErrors(func() { got, err = service.UnmergedWork(blockers) })
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if !maps.EqualFunc(got, testCase.want, slices.Equal) {
				t.Errorf("expected the unmerged work %v, got %v", testCase.want, got)
			}
		})
	}
}
