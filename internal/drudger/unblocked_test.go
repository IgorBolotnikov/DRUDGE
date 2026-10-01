package drudger

import (
	"errors"
	"path/filepath"
	"slices"
	"testing"

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

// finish ends the work on finished with the outcome given.
func (finish unblockedFinish) finish(t *testing.T, service *testService, finished *task.Task, outcome task.TaskStatus) {
	t.Helper()

	var err error
	switch {
	case finish.isMarkDone:
		_, err = service.MarkDone(testProjectSlug, finished.ID)
	case finish.isByHand:
		changes := task.EditTaskDto{Status: &outcome, AllowsManagedStatus: true}
		_, err = service.EditTask(testProjectSlug, finished.ID, changes)
	default:
		service.runs.writeStream(finished.ID, initEvent, resultEvent)
		exitCode := "0\n"
		if outcome == task.StatusFuckedUp {
			exitCode = "1\n"
		}
		service.runs.writeExit(finished.ID, exitCode)
		_, err = service.SessionStatus(testProjectSlug, finished.ID)
	}

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if finished.Status != outcome {
		t.Fatalf("expected the task to end %q, got %q", outcome, finished.Status)
	}
}

func TestDrudgerService_ReportsWhatAFinishedTaskUnblocked(t *testing.T) {
	cases := []struct {
		name    string
		outcome task.TaskStatus
		// others are the tasks stored next to the finished one.
		others []*task.Task
		// commits is how many commits the run of the finished task left.
		commits int
		// wantUnblocked are the dependents the finish reports as runnable, and
		// none means it reports nothing about them.
		wantUnblocked []task.TaskID
	}{
		{
			name:    "a task nothing waits for",
			outcome: task.StatusDone,
			others:  []*task.Task{blockerTask("4f2a1b3c-0001", task.StatusTodo, "Wire the repository")},
		},
		{
			name:          "one dependent that became runnable",
			outcome:       task.StatusDone,
			others:        []*task.Task{dependentOf("4f2a1b3c-0001", "Wire the repository")},
			wantUnblocked: []task.TaskID{"4f2a1b3c-0001"},
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
			wantUnblocked: []task.TaskID{"4f2a1b3c-0001"},
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
				service := newTestServiceWithPool(settingsWith(testRepositoryName), &fakeCommandRunner{}, pool, stored...)
				if testCase.commits > 0 {
					service.git.leaveOn(worktree, testTaskBranch, testHeadSHA)
					service.git.commitsOn(testHeadSHA, testCase.commits)
				} else {
					service.git.leaveOn(worktree, testTaskBranch, testBaseSHA)
				}

				finish.finish(t, service, finished, testCase.outcome)

				reported := reportedEvents[DependentsUnblocked](service.progress)
				if len(testCase.wantUnblocked) == 0 {
					if len(reported) != 0 {
						t.Errorf("expected nothing about dependents, got %+v", reported)
					}
					return
				}
				// The dependents are reported after everything else the finish
				// reports.
				events := service.progress.events
				last, ok := events[len(events)-1].(DependentsUnblocked)
				if len(reported) != 1 || !ok {
					t.Fatalf("expected the dependents to be reported once and last, got %+v", events)
				}
				unblocked := make([]task.TaskID, 0, len(last.Tasks))
				for _, dependent := range last.Tasks {
					unblocked = append(unblocked, dependent.ID)
				}
				if !slices.Equal(unblocked, testCase.wantUnblocked) {
					t.Errorf("expected the dependents %v to be reported, got %v", testCase.wantUnblocked, unblocked)
				}
			})
		}
	}
}

func TestDrudgerService_MarkDone_ReportsDependentsItCouldNotWorkOut(t *testing.T) {
	setupProjectDir(t)
	finished := todoTask()
	finished.Status = task.StatusUnmerged

	service := newTestServiceWithPool(settingsWith(testRepositoryName), &fakeCommandRunner{}, nil, finished)
	listErr := errors.New("disk full")
	service.taskRepo.listErr = listErr

	marked, err := service.MarkDone(testProjectSlug, finished.ID)
	if err != nil {
		t.Fatalf("expected the task to be marked done, got %v", err)
	}
	if marked.Status != task.StatusDone {
		t.Errorf("expected status %q, got %q", task.StatusDone, marked.Status)
	}

	failed := singleEvent[UnblockedLookupFailed](t, service.progress)
	if failed.TaskID != finished.ID || !errors.Is(failed.Err, listErr) {
		t.Errorf("expected a failed lookup of the dependents of task %s with %v, got %+v", finished.ID, listErr, failed)
	}
}

// dependentOf is a todo task blocked by the task todoTask returns and by the
// other blockers named.
func dependentOf(id task.TaskID, title string, otherBlockers ...task.TaskID) *task.Task {
	dependent := blockerTask(id, task.StatusTodo, title)
	dependent.BlockedBy = append([]task.TaskID{todoTask().ID}, otherBlockers...)
	return dependent
}
