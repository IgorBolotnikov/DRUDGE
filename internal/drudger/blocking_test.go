package drudger

import (
	"errors"
	"io/fs"
	"os"
	"strings"
	"testing"

	"drudge/internal/common"
	"drudge/internal/config"
	"drudge/internal/task"
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
