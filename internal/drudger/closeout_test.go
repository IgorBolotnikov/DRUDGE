package drudger

import (
	"maps"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/IgorBolotnikov/DRUDGE/internal/common"
	"github.com/IgorBolotnikov/DRUDGE/internal/config"
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
		// wantDeletedIn names the repositories whose empty branch is deleted.
		wantDeletedIn []string
		// wantCreated are the branches close-out made.
		wantCreated []string
	}{
		{
			name: "a run that committed",
			leave: func(fake *fakeGit, worktrees map[string]string) {
				fake.leaveOn(worktrees[testRepositoryName], testTaskBranch, testHeadSHA)
				fake.commitsOn(testHeadSHA, 3)
			},
			wantLandings: map[string]task.Landing{
				testRepositoryName: {Branch: testTaskBranch, Base: testBaseSHA, Head: testHeadSHA, Commits: 3},
			},
		},
		{
			name: "a run that committed nothing",
			leave: func(fake *fakeGit, worktrees map[string]string) {
				fake.leaveOn(worktrees[testRepositoryName], testTaskBranch, testBaseSHA)
			},
			wantDeletedIn: []string{testRepositoryName},
		},
		{
			name: "an agent that ended on another branch",
			leave: func(fake *fakeGit, worktrees map[string]string) {
				fake.leaveOn(worktrees[testRepositoryName], testSideBranch, testHeadSHA)
				fake.commitsOn(testHeadSHA, 2)
			},
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
			wantLandings: map[string]task.Landing{
				"api": {Branch: testTaskBranch, Base: testBaseSHA, Head: testHeadSHA, Commits: 4},
			},
			wantDeletedIn: []string{"ui"},
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
			writeStream(t, common.RunDir(projectDir, string(tracked.ID)), initEvent, resultEvent)
			writeExit(t, common.RunDir(projectDir, string(tracked.ID)), "0\n")

			pool := []*Drudger{holdingDrudger(projectDir, tracked.ID)}
			service := newTestServiceWithPool(localConfigWith(repositories...), config.DefaultConfig(), &fakeCommandRunner{}, pool, tracked)
			worktrees := worktreesOf(projectDir, repositories)
			testCase.leave(service.git, worktrees)

			var session *TaskSession
			var err error
			captureOutput(func() { session, err = service.SessionStatus(testProjectSlug, tracked.ID) })
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if !maps.Equal(session.Task.Landings, testCase.wantLandings) {
				t.Errorf("expected the landings %v, got %v", testCase.wantLandings, session.Task.Landings)
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
		})
	}
}

func TestDrudgerService_SessionStatus_KeepsTheHandoverWhenNoDrudgerHoldsTheTask(t *testing.T) {
	projectDir := setupProjectDir(t)
	tracked := handedOverTask(testRepositoryName)
	writeStream(t, common.RunDir(projectDir, string(tracked.ID)), initEvent, resultEvent)
	writeExit(t, common.RunDir(projectDir, string(tracked.ID)), "0\n")

	service := newTestServiceWithPool(localConfigWith(testRepositoryName), config.DefaultConfig(), &fakeCommandRunner{}, nil, tracked)

	var session *TaskSession
	var err error
	output := captureOutput(func() { session, err = service.SessionStatus(testProjectSlug, tracked.ID) })
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if session.Task.Status != task.StatusDone {
		t.Errorf("expected the outcome to be recorded anyway, got %q", session.Task.Status)
	}
	want := map[string]task.Landing{testRepositoryName: {Branch: testTaskBranch, Base: testBaseSHA}}
	if !maps.Equal(session.Task.Landings, want) {
		t.Errorf("expected the task to keep what the handover recorded, got %v", session.Task.Landings)
	}
	if !strings.Contains(output, string(tracked.ID)) {
		t.Errorf("expected the task to be named in the warning, got %q", output)
	}
}

func TestDrudgerService_SessionStatus_ARefusedRunClosesOutNothing(t *testing.T) {
	projectDir := setupProjectDir(t)
	tracked := handedOverTask(testRepositoryName)
	writeStream(t, common.RunDir(projectDir, string(tracked.ID)), initEvent, authRefusedEvent, authRefusedResultEvent)
	writeExit(t, common.RunDir(projectDir, string(tracked.ID)), "1\n")

	pool := []*Drudger{holdingDrudger(projectDir, tracked.ID)}
	service := newTestServiceWithPool(localConfigWith(testRepositoryName), config.DefaultConfig(), &fakeCommandRunner{}, pool, tracked)
	service.git.leaveOn(filepath.Join(slotRoot(projectDir, 1), testRepositoryName), testTaskBranch, testBaseSHA)

	var err error
	captureOutput(func() { _, err = service.SessionStatus(testProjectSlug, tracked.ID) })
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
