package drudger

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/IgorBolotnikov/DRUDGE/internal/common"
	"github.com/IgorBolotnikov/DRUDGE/internal/config"
	"github.com/IgorBolotnikov/DRUDGE/internal/task"
)

// unblockedFinish is one of the ways a task reaches the end of its work.
type unblockedFinish struct {
	name string
	// startsFrom is the status the finished task has before the finish.
	startsFrom task.TaskStatus
	isByHand   bool
	isMarkDone bool
}

var unblockedFinishes = []unblockedFinish{
	{name: "Session check", startsFrom: task.StatusInProgress},
	{name: "hand edit", startsFrom: task.StatusFuckedUp, isByHand: true},
	{name: "task done", startsFrom: task.StatusUnmerged, isMarkDone: true},
}

// canReach reports whether this finish can end the work with the outcome
// given. Marking a task done only ever makes it done.
func (finish unblockedFinish) canReach(outcome task.TaskStatus) bool {
	return !finish.isMarkDone || outcome == task.StatusDone
}

// finish ends the work on finished with the outcome given and returns what the
// command printed.
func (finish unblockedFinish) finish(t *testing.T, service *testService, projectDir string, finished *task.Task, outcome task.TaskStatus) string {
	t.Helper()

	var err error
	var output string
	switch {
	case finish.isMarkDone:
		output = captureOutput(func() { _, err = service.MarkDone(testProjectSlug, finished.ID) })
	case finish.isByHand:
		changes := task.EditTaskDto{Status: &outcome, AllowsManagedStatus: true}
		output = captureOutput(func() { _, err = service.EditTask(testProjectSlug, finished.ID, changes) })
	default:
		runDir := common.RunDir(projectDir, string(finished.ID))
		writeStream(t, runDir, initEvent, resultEvent)
		exitCode := "0\n"
		if outcome == task.StatusFuckedUp {
			exitCode = "1\n"
		}
		writeExit(t, runDir, exitCode)
		output = captureOutput(func() { _, err = service.SessionStatus(testProjectSlug, finished.ID) })
	}

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if finished.Status != outcome {
		t.Fatalf("expected the task to end %q, got %q", outcome, finished.Status)
	}
	return output
}

func TestDrudgerService_ReportsWhatAFinishedTaskUnblocked(t *testing.T) {
	cases := []struct {
		name    string
		outcome task.TaskStatus
		// others are the tasks stored next to the finished one.
		others []*task.Task
		// commits is how many commits the run of the finished task left.
		commits int
		// wantLines are what the command prints about the dependents, and none
		// means it says nothing about them.
		wantLines []string
	}{
		{
			name:    "a task nothing waits for",
			outcome: task.StatusDone,
			others:  []*task.Task{blockerTask("4f2a1b3c-0001", task.StatusTodo, "Wire the repository")},
		},
		{
			name:    "one dependent that became runnable",
			outcome: task.StatusDone,
			others:  []*task.Task{dependentOf("4f2a1b3c-0001", "Wire the repository")},
			wantLines: []string{
				"It unblocked 1 task:",
				"  4f2a1b3c  Wire the repository",
			},
		},
		{
			name:    "a dependent still held by another blocker",
			outcome: task.StatusDone,
			others: []*task.Task{
				dependentOf("4f2a1b3c-0001", "Wire the repository", "1a2b3c4d-0003"),
				blockerTask("1a2b3c4d-0003", task.StatusInProgress, "Refuse a cycle"),
			},
		},
		{
			name:    "a runnable dependent next to a held one",
			outcome: task.StatusDone,
			others: []*task.Task{
				dependentOf("4f2a1b3c-0001", "Wire the repository"),
				dependentOf("7e6d5c4b-0002", "Add the endpoint", "1a2b3c4d-0003"),
				blockerTask("1a2b3c4d-0003", task.StatusTodo, "Refuse a cycle"),
			},
			wantLines: []string{
				"It unblocked 1 task:",
				"  4f2a1b3c  Wire the repository",
			},
		},
		{
			name:    "a task whose work is not merged",
			outcome: task.StatusUnmerged,
			others:  []*task.Task{dependentOf("4f2a1b3c-0001", "Wire the repository")},
			commits: 3,
		},
		{
			name:    "a task that fucked up",
			outcome: task.StatusFuckedUp,
			others:  []*task.Task{dependentOf("4f2a1b3c-0001", "Wire the repository")},
			commits: 3,
		},
	}

	for _, testCase := range cases {
		for _, finish := range unblockedFinishes {
			if !finish.canReach(testCase.outcome) {
				continue
			}
			t.Run(testCase.name+" on a "+finish.name, func(t *testing.T) {
				projectDir := setupProjectDir(t)
				finished := handedOverTask(testRepositoryName)
				worktree := filepath.Join(slotRoot(projectDir, 1), testRepositoryName)
				finished.Status = finish.startsFrom
				var pool []*Drudger

				if finish.isByHand || finish.isMarkDone {
					finished.Landings = nil
					if testCase.commits > 0 {
						finished.RecordLanding(testRepositoryName, task.Landing{Branch: testTaskBranch, Base: testBaseSHA, Head: testHeadSHA, Commits: testCase.commits})
					}
				} else {
					pool = []*Drudger{holdingDrudger(projectDir, finished.ID)}
				}

				stored := append([]*task.Task{finished}, testCase.others...)
				service := newTestServiceWithPool(localConfigWith(testRepositoryName), config.DefaultConfig(), &fakeCommandRunner{}, pool, stored...)
				if testCase.commits > 0 {
					service.git.leaveOn(worktree, testTaskBranch, testHeadSHA)
					service.git.commitsOn(testHeadSHA, testCase.commits)
				} else {
					service.git.leaveOn(worktree, testTaskBranch, testBaseSHA)
				}

				output := finish.finish(t, service, projectDir, finished, testCase.outcome)

				if len(testCase.wantLines) == 0 {
					if strings.Contains(output, "unblocked") {
						t.Errorf("expected nothing about dependents, got\n%s", output)
					}
					return
				}
				if want := strings.Join(testCase.wantLines, "\n") + "\n"; !strings.HasSuffix(output, want) {
					t.Errorf("expected the output to end with\n%s\ngot\n%s", want, output)
				}
			})
		}
	}
}

// dependentOf is a todo task blocked by the task todoTask returns and by the
// other blockers named.
func dependentOf(id task.TaskID, title string, otherBlockers ...task.TaskID) *task.Task {
	dependent := blockerTask(id, task.StatusTodo, title)
	dependent.BlockedBy = append([]task.TaskID{todoTask().ID}, otherBlockers...)
	return dependent
}
