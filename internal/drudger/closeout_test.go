package drudger

import (
	"errors"
	"maps"
	"path/filepath"
	"slices"
	"testing"

	"github.com/IgorBolotnikov/DRUDGE/internal/task"
)

// testHeadSHA is where a worktree sits once its agent has committed.
const testHeadSHA = "1a2b3c4d5e6f708192a3b4c5d6e7f8091a2b3c4d"

// testRescueBranch is the branch close-out puts on commits no branch reaches.
const testRescueBranch = testTaskBranch + rescueBranchSuffix

// testSideBranch is a branch an agent made for itself.
const testSideBranch = "feature/side"

func TestDrudgerService_SessionStatus_RecordsWhereTheWorkLanded(t *testing.T) {
	cases := []struct {
		name string
		// repositories are the repositories of the project. An empty list
		// stands for the single repository most cases work with.
		repositories []string
		// leave puts the worktrees in the state the agent left them behind in,
		// keyed by repository name.
		leave func(fake *fakeGit, worktrees map[string]string)

		wantLandings map[string]task.Landing
		wantStatus   task.TaskStatus
		// wantDeletedIn names the repositories whose empty branch is deleted.
		wantDeletedIn []string
		// wantCreated are the branches close-out made.
		wantCreated []string
		// wantFailedIn names the repositories whose close-out is reported as
		// failed.
		wantFailedIn []string
	}{
		{
			name: "a run that committed",
			leave: func(fake *fakeGit, worktrees map[string]string) {
				fake.leaveOn(worktrees[testRepositoryName], testTaskBranch, testHeadSHA)
				fake.commitsOn(testHeadSHA, 3)
			},
			wantStatus: task.StatusUnmerged,
			wantLandings: map[string]task.Landing{
				testRepositoryName: {Branch: testTaskBranch, Base: testBaseSHA, Head: testHeadSHA, Commits: 3},
			},
		},
		{
			name: "a run that committed nothing",
			leave: func(fake *fakeGit, worktrees map[string]string) {
				fake.leaveOn(worktrees[testRepositoryName], testTaskBranch, testBaseSHA)
			},
			wantStatus:    task.StatusDone,
			wantDeletedIn: []string{testRepositoryName},
		},
		{
			name: "an agent that ended on another branch",
			leave: func(fake *fakeGit, worktrees map[string]string) {
				fake.leaveOn(worktrees[testRepositoryName], testSideBranch, testHeadSHA)
				fake.commitsOn(testHeadSHA, 2)
			},
			wantStatus: task.StatusUnmerged,
			wantLandings: map[string]task.Landing{
				testRepositoryName: {Branch: testSideBranch, Base: testBaseSHA, Head: testHeadSHA, Commits: 2},
			},
		},
		{
			name: "an agent on a detached HEAD with commits no branch reaches",
			leave: func(fake *fakeGit, worktrees map[string]string) {
				fake.leaveDetached(worktrees[testRepositoryName], testHeadSHA)
				fake.commitsOn(testHeadSHA, 1)
			},
			wantStatus: task.StatusUnmerged,
			wantLandings: map[string]task.Landing{
				testRepositoryName: {Branch: testRescueBranch, Base: testBaseSHA, Head: testHeadSHA, Commits: 1},
			},
			wantCreated: []string{testRescueBranch},
		},
		{
			name: "an agent on a detached HEAD its branch still reaches",
			leave: func(fake *fakeGit, worktrees map[string]string) {
				fake.leaveDetached(worktrees[testRepositoryName], testHeadSHA, testTaskBranch)
				fake.commitsOn(testHeadSHA, 1)
			},
			wantStatus: task.StatusUnmerged,
			wantLandings: map[string]task.Landing{
				testRepositoryName: {Branch: testTaskBranch, Base: testBaseSHA, Head: testHeadSHA, Commits: 1},
			},
		},
		{
			name: "a worktree detached after the run keeps what its branch holds",
			leave: func(fake *fakeGit, worktrees map[string]string) {
				fake.leaveDetached(worktrees[testRepositoryName], testBaseSHA)
				fake.branchAt(worktrees[testRepositoryName], testTaskBranch, testHeadSHA)
				fake.commitsOn(testHeadSHA, 2)
			},
			wantStatus: task.StatusUnmerged,
			wantLandings: map[string]task.Landing{
				testRepositoryName: {Branch: testTaskBranch, Base: testBaseSHA, Head: testHeadSHA, Commits: 2},
			},
		},
		{
			name:         "a run that committed in one repository out of two",
			repositories: []string{"api", "ui"},
			leave: func(fake *fakeGit, worktrees map[string]string) {
				fake.leaveOn(worktrees["api"], testTaskBranch, testHeadSHA)
				fake.commitsOn(testHeadSHA, 4)
				fake.leaveOn(worktrees["ui"], testTaskBranch, testBaseSHA)
			},
			wantStatus: task.StatusUnmerged,
			wantLandings: map[string]task.Landing{
				"api": {Branch: testTaskBranch, Base: testBaseSHA, Head: testHeadSHA, Commits: 4},
			},
			wantDeletedIn: []string{"ui"},
		},
		{
			name: "a repository close-out cannot read keeps the handover",
			leave: func(fake *fakeGit, worktrees map[string]string) {
				fake.leaveDetached(worktrees[testRepositoryName], testHeadSHA)
				fake.commitsOn(testHeadSHA, 1)
				fake.branchErr = errors.New("cannot lock ref")
			},
			wantLandings: map[string]task.Landing{
				testRepositoryName: {Branch: testTaskBranch, Base: testBaseSHA},
			},
			wantStatus:   task.StatusUnmerged,
			wantFailedIn: []string{testRepositoryName},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			repositories := testCase.repositories
			if len(repositories) == 0 {
				repositories = []string{testRepositoryName}
			}

			projectDir := setupProjectDir(t)
			tracked := handedOverTask(repositories...)

			pool := []*Drudger{holdingDrudger(projectDir, tracked.ID)}
			service := newTestServiceWithPool(settingsWith(repositories...), &fakeCommandRunner{}, pool, tracked)
			service.runs.writeStream(tracked.ID, initEvent, resultEvent)
			service.runs.writeExit(tracked.ID, "0\n")
			worktrees := worktreesOf(projectDir, repositories)
			testCase.leave(service.git, worktrees)

			var session *TaskSession
			var err error
			session, err = service.SessionStatus(testProjectSlug, tracked.ID)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if !maps.Equal(session.Task.Landings, testCase.wantLandings) {
				t.Errorf("expected the landings %v, got %v", testCase.wantLandings, session.Task.Landings)
			}
			if session.Task.Status != testCase.wantStatus {
				t.Errorf("expected status %q, got %q", testCase.wantStatus, session.Task.Status)
			}

			wantDeleted := make([]branchRef, 0, len(testCase.wantDeletedIn))
			for _, repository := range testCase.wantDeletedIn {
				wantDeleted = append(wantDeleted, branchRef{dir: worktrees[repository], branch: testTaskBranch})
			}
			if !slices.Equal(service.git.deletedBranches, wantDeleted) {
				t.Errorf("expected the branches %v to be deleted, got %v", wantDeleted, service.git.deletedBranches)
			}

			if got := branchesOf(service.git.createdBranches); !slices.Equal(got, testCase.wantCreated) {
				t.Errorf("expected close-out to make the branches %v, got %v", testCase.wantCreated, got)
			}

			var failedIn []string
			for _, failed := range reportedEvents[RunCloseOutFailed](service.progress) {
				if failed.TaskID != tracked.ID || failed.Step != RepositoryCloseOutStep || failed.Err == nil {
					t.Errorf("expected a failed close-out of a repository of task %s, got %+v", tracked.ID, failed)
				}
				failedIn = append(failedIn, failed.Repository)
			}
			if !slices.Equal(failedIn, testCase.wantFailedIn) {
				t.Errorf("expected the close-out to fail in %v, got %v", testCase.wantFailedIn, failedIn)
			}
		})
	}
}

func TestDrudgerService_SessionStatus_KeepsTheHandoverWhenNoDrudgerHoldsTheTask(t *testing.T) {
	setupProjectDir(t)
	tracked := handedOverTask(testRepositoryName)

	service := newTestServiceWithPool(settingsWith(testRepositoryName), &fakeCommandRunner{}, nil, tracked)
	service.runs.writeStream(tracked.ID, initEvent, resultEvent)
	service.runs.writeExit(tracked.ID, "0\n")

	session, err := service.SessionStatus(testProjectSlug, tracked.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if session.Task.Status != task.StatusUnmerged {
		t.Errorf("expected a clean run that kept its handover to be %q, got %q", task.StatusUnmerged, session.Task.Status)
	}
	want := map[string]task.Landing{testRepositoryName: {Branch: testTaskBranch, Base: testBaseSHA}}
	if !maps.Equal(session.Task.Landings, want) {
		t.Errorf("expected the task to keep what the handover recorded, got %v", session.Task.Landings)
	}
	if failed := singleEvent[RunCloseOutFailed](t, service.progress); failed.TaskID != tracked.ID || failed.Step != WorkspaceReadStep || failed.Err == nil {
		t.Errorf("expected a failed read of the workspace of task %s, got %+v", tracked.ID, failed)
	}
}

func TestDrudgerService_SessionStatus_ReportsAWorkspaceItCouldNotPark(t *testing.T) {
	projectDir := setupProjectDir(t)
	tracked := handedOverTask(testRepositoryName)

	pool := []*Drudger{holdingDrudger(projectDir, tracked.ID)}
	service := newTestServiceWithPool(settingsWith(testRepositoryName), &fakeCommandRunner{}, pool, tracked)
	service.runs.writeStream(tracked.ID, initEvent, resultEvent)
	service.runs.writeExit(tracked.ID, "0\n")
	worktree := filepath.Join(slotRoot(projectDir, 1), testRepositoryName)
	makeWorktrees(t, map[string]string{testRepositoryName: worktree})
	service.git.leaveOn(worktree, testTaskBranch, testHeadSHA)
	service.git.commitsOn(testHeadSHA, 1)
	service.git.leaveDirty(worktree)
	stashErr := errors.New("git said no")
	service.git.stashErr = stashErr

	if _, err := service.SessionStatus(testProjectSlug, tracked.ID); err != nil {
		t.Fatalf("expected the check to go through, got %v", err)
	}

	failed := singleEvent[RunCloseOutFailed](t, service.progress)
	if failed.TaskID != tracked.ID || failed.Step != WorkspaceParkStep || !errors.Is(failed.Err, stashErr) {
		t.Errorf("expected a failed park of the workspace of task %s with %v, got %+v", tracked.ID, stashErr, failed)
	}
}

func TestDrudgerService_SessionStatus_ARefusedRunClosesOutNothing(t *testing.T) {
	projectDir := setupProjectDir(t)
	tracked := handedOverTask(testRepositoryName)

	pool := []*Drudger{holdingDrudger(projectDir, tracked.ID)}
	service := newTestServiceWithPool(settingsWith(testRepositoryName), &fakeCommandRunner{}, pool, tracked)
	service.runs.writeStream(tracked.ID, initEvent, authRefusedEvent, authRefusedResultEvent)
	service.runs.writeExit(tracked.ID, "1\n")
	service.git.leaveOn(filepath.Join(slotRoot(projectDir, 1), testRepositoryName), testTaskBranch, testBaseSHA)

	var err error
	_, err = service.SessionStatus(testProjectSlug, tracked.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(service.git.deletedBranches) != 0 {
		t.Errorf("expected a refused run to delete no branch, got %v", service.git.deletedBranches)
	}
}

// handedOverTask is a task an agent is working on, carrying what the handover
// recorded in each repository.
func handedOverTask(repositories ...string) *task.Task {
	tracked := todoTask()
	tracked.Status = task.StatusInProgress
	for _, repository := range repositories {
		tracked.RecordLanding(repository, task.Landing{Branch: testTaskBranch, Base: testBaseSHA})
	}
	return tracked
}

// holdingDrudger is the Drudger of slot 1 with a task on it.
func holdingDrudger(projectDir string, taskID task.TaskID) *Drudger {
	return &Drudger{Slot: 1, Sandbox: testSandbox, Workspace: slotRoot(projectDir, 1), TaskID: taskID}
}

// worktreesOf maps each repository of a project to the worktree slot 1 checks
// it out in.
func worktreesOf(projectDir string, repositories []string) map[string]string {
	worktrees := make(map[string]string, len(repositories))
	for _, repository := range repositories {
		worktrees[repository] = filepath.Join(slotRoot(projectDir, 1), repository)
	}
	return worktrees
}

// leaveOn puts a worktree on a branch whose tip is at a commit.
func (fake *fakeGit) leaveOn(worktree string, branch string, tip string) {
	fake.rememberBranch(worktree, branch)
	fake.setCommit(worktree, branch, tip)
	fake.setHead(worktree, tip)
}

// leaveDetached puts a worktree at a commit with no branch on it. The branches
// named are the ones holding that commit.
func (fake *fakeGit) leaveDetached(worktree string, head string, reaching ...string) {
	fake.setHead(worktree, head)
	for _, branch := range reaching {
		fake.branchAt(worktree, branch, head)
	}
	if len(reaching) > 0 {
		if fake.reachingBranches == nil {
			fake.reachingBranches = map[string][]string{}
		}
		fake.reachingBranches[head] = reaching
	}
}

// branchAt gives a worktree's repository a branch with its tip at a commit,
// checked out nowhere.
func (fake *fakeGit) branchAt(worktree string, branch string, tip string) {
	fake.registerBranch(worktree, branch)
	fake.setCommit(worktree, branch, tip)
}

// commitsOn says how many commits a ref holds beyond the base it was cut from.
func (fake *fakeGit) commitsOn(ref string, count int) {
	if fake.branchCommits == nil {
		fake.branchCommits = map[string]int{}
	}
	fake.branchCommits[ref] = count
}
