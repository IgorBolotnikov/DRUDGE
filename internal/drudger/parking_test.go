package drudger

import (
	"path/filepath"
	"slices"
	"testing"
	"time"

	"drudge/internal/common"
	"drudge/internal/config"
	"drudge/internal/git"
)

func TestDrudgerService_SessionStatus_ParksTheWorkspace(t *testing.T) {
	cases := []struct {
		name string
		// An empty list stands for the single repository most cases work with.
		repositories []string
		// leave puts the worktrees in the state the agent left them in, keyed
		// by repository name.
		leave func(fake *fakeGit, worktrees map[string]string)

		// wantDetachedIn names the repositories parking detaches, and
		// wantStashedIn the ones it stashes.
		wantDetachedIn []string
		wantStashedIn  []string
	}{
		{
			name: "a run that committed leaves its worktree detached",
			leave: func(fake *fakeGit, worktrees map[string]string) {
				fake.leaveOn(worktrees[testRepositoryName], testTaskBranch, testHeadSHA)
				fake.commitsOn(testHeadSHA, 2)
			},
			wantDetachedIn: []string{testRepositoryName},
		},
		{
			name: "a dirty worktree is stashed before it is detached",
			leave: func(fake *fakeGit, worktrees map[string]string) {
				fake.leaveOn(worktrees[testRepositoryName], testTaskBranch, testHeadSHA)
				fake.commitsOn(testHeadSHA, 2)
				fake.leaveDirty(worktrees[testRepositoryName])
			},
			wantDetachedIn: []string{testRepositoryName},
			wantStashedIn:  []string{testRepositoryName},
		},
		{
			name: "a workspace that is already parked is left alone",
			leave: func(fake *fakeGit, worktrees map[string]string) {
				fake.leaveDetached(worktrees[testRepositoryName], testBaseSHA)
			},
		},
		{
			name:         "every repository of a workspace is parked",
			repositories: []string{"api", "ui"},
			leave: func(fake *fakeGit, worktrees map[string]string) {
				fake.leaveOn(worktrees["api"], testTaskBranch, testHeadSHA)
				fake.leaveOn(worktrees["ui"], testTaskBranch, testHeadSHA)
				fake.commitsOn(testHeadSHA, 2)
				fake.leaveDirty(worktrees["ui"])
			},
			wantDetachedIn: []string{"api", "ui"},
			wantStashedIn:  []string{"ui"},
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
			runDir := common.RunDir(projectDir, string(tracked.ID))
			writeStream(t, runDir, initEvent, resultEvent)
			writeExit(t, runDir, "0\n")

			pool := []*Drudger{holdingDrudger(projectDir, tracked.ID)}
			service := newTestServiceWithPool(localConfigWith(repositories...), config.DefaultConfig(), &fakeCommandRunner{}, pool, tracked)
			worktrees := worktreesOf(projectDir, repositories)
			makeWorktrees(t, worktrees)
			testCase.leave(service.git, worktrees)

			var session *TaskSession
			var err error
			captureOutput(func() { session, err = service.SessionStatus(testProjectSlug, tracked.ID) })
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			wantDetached := pathsOf(worktrees, testCase.wantDetachedIn)
			if !slices.Equal(service.git.detachedWorktrees, wantDetached) {
				t.Errorf("expected the worktrees %v to be detached, got %v", wantDetached, service.git.detachedWorktrees)
			}

			wantStashed := pathsOf(worktrees, testCase.wantStashedIn)
			if got := stashedDirs(service.git.stashes); !slices.Equal(got, wantStashed) {
				t.Errorf("expected the worktrees %v to be stashed, got %v", wantStashed, got)
			}
			for index, repository := range testCase.wantStashedIn {
				want := service.git.stashes[index].sha
				if got := session.Task.Stashes[repository]; got != want {
					t.Errorf("expected repository %s to record stash %s, got %q", repository, want, got)
				}
			}
		})
	}
}

func TestDrudgerService_SessionStatus_LeavesTheWorkspaceOfALiveSession(t *testing.T) {
	projectDir := setupProjectDir(t)
	tracked := handedOverTask(testRepositoryName)
	writeStream(t, common.RunDir(projectDir, string(tracked.ID)), initEvent, assistantEvent)

	pool := []*Drudger{holdingDrudger(projectDir, tracked.ID)}
	service := newTestServiceWithPool(localConfigWith(testRepositoryName), config.DefaultConfig(), &fakeCommandRunner{}, pool, tracked)
	worktrees := worktreesOf(projectDir, []string{testRepositoryName})
	makeWorktrees(t, worktrees)
	service.git.leaveOn(worktrees[testRepositoryName], testTaskBranch, testHeadSHA)
	service.git.leaveDirty(worktrees[testRepositoryName])

	var err error
	captureOutput(func() { _, err = service.SessionStatus(testProjectSlug, tracked.ID) })
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(service.git.detachedWorktrees) != 0 {
		t.Errorf("expected the worktree of a working agent to keep its branch, got %v", service.git.detachedWorktrees)
	}
	if len(service.git.stashes) != 0 {
		t.Errorf("expected the worktree of a working agent to be left alone, got %v", service.git.stashes)
	}
}

func TestDrudgerService_ReclaimDrudgers_ParksEveryIdleDrudger(t *testing.T) {
	projectDir := setupProjectDir(t)

	working := busyDrudger(1)
	working.Workspace = slotRoot(projectDir, 1)
	working.LastChecked = time.Now().UTC().Add(-time.Hour)
	writeStream(t, common.RunDir(projectDir, string(working.TaskID)), initEvent, assistantEvent)

	idle := idleDrudgerAt(projectDir, 2)

	commands := &fakeCommandRunner{projectDir: projectDir, outputs: []string{sandboxListingWith(testSandbox)}}
	pool := []*Drudger{working, idle}
	service := newTestServiceWithPool(localConfigWith(testRepositoryName), config.DefaultConfig(), commands, pool)

	worktrees := map[string]string{"working": slotWorktree(projectDir, 1), "idle": slotWorktree(projectDir, 2)}
	makeWorktrees(t, worktrees)
	service.git.leaveOn(worktrees["working"], testTaskBranch, testHeadSHA)
	service.git.leaveOn(worktrees["idle"], testTaskBranch, testHeadSHA)
	service.git.leaveDirty(worktrees["idle"])

	var err error
	captureOutput(func() { _, err = service.ReclaimDrudgers(testProjectSlug) })
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	wantDetached := []string{worktrees["idle"]}
	if !slices.Equal(service.git.detachedWorktrees, wantDetached) {
		t.Errorf("expected only the idle Drudger to be detached, got %v", service.git.detachedWorktrees)
	}
	if got := stashedDirs(service.git.stashes); !slices.Equal(got, wantDetached) {
		t.Errorf("expected the idle Drudger to be stashed, got %v", got)
	}
}

func TestDrudgerService_ReclaimDrudgers_ParksNoDrudgerWithoutAWorkspace(t *testing.T) {
	setupProjectDir(t)

	commands := &fakeCommandRunner{outputs: []string{sandboxListingWith()}}
	pool := []*Drudger{idleDrudger(1)}
	service := newTestServiceWithPool(localConfigWith(testRepositoryName), config.DefaultConfig(), commands, pool)
	service.gitOps = &refusingGit{t: t}

	var err error
	captureOutput(func() { _, err = service.ReclaimDrudgers(testProjectSlug) })
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDrudgerService_ListDrudgers_RunsNoGitCommands(t *testing.T) {
	projectDir := setupProjectDir(t)

	claimed := busyDrudger(1)
	claimed.Workspace = slotRoot(projectDir, 1)
	finishSession(t, projectDir, claimed.TaskID)

	service := newTestServiceWithPool(localConfigWith(testRepositoryName), config.DefaultConfig(), &fakeCommandRunner{}, []*Drudger{claimed})
	service.gitOps = &refusingGit{t: t}

	var err error
	captureOutput(func() { _, err = service.ListDrudgers(testProjectSlug) })
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !service.drudgers.atSlot(1).Idle() {
		t.Error("expected the finished Session to free its slot")
	}
}

// idleDrudgerAt is an idle Drudger with a workspace of its own.
func idleDrudgerAt(projectDir string, slot int) *Drudger {
	return &Drudger{Slot: slot, Sandbox: testSandboxOfSlot(slot), Workspace: slotRoot(projectDir, slot)}
}

// slotWorktree is where the Drudger of a slot checks the single repository of
// the test project out.
func slotWorktree(projectDir string, slot int) string {
	return filepath.Join(slotRoot(projectDir, slot), testRepositoryName)
}

// makeWorktrees creates the worktree directories of a workspace. Parking skips
// a repository whose worktree is not on disk.
func makeWorktrees(t *testing.T, worktrees map[string]string) {
	t.Helper()
	for _, worktree := range worktrees {
		if err := common.EnsureDir(worktree); err != nil {
			t.Fatalf("could not create the worktree %s: %v", worktree, err)
		}
	}
}

// pathsOf returns the worktree of each named repository.
func pathsOf(worktrees map[string]string, repositories []string) []string {
	paths := make([]string, 0, len(repositories))
	for _, repository := range repositories {
		paths = append(paths, worktrees[repository])
	}
	return paths
}

// stashedDirs returns where each stash was made, in the order they were made.
func stashedDirs(stashes []stashCall) []string {
	dirs := make([]string, 0, len(stashes))
	for _, stash := range stashes {
		dirs = append(dirs, stash.dir)
	}
	return dirs
}

// leaveDirty says a worktree holds uncommitted changes.
func (fake *fakeGit) leaveDirty(worktree string) {
	if fake.dirtyWorktrees == nil {
		fake.dirtyWorktrees = map[string]bool{}
	}
	fake.dirtyWorktrees[worktree] = true
}

// refusingGit fails the test on every call. It stands for an operation that
// must run no git commands.
type refusingGit struct {
	t *testing.T
}

func (fake *refusingGit) refuse(operation string) error {
	fake.t.Helper()
	fake.t.Errorf("expected no git command, got %s", operation)
	return nil
}

func (fake *refusingGit) IsRepositoryRoot(dir string) (bool, error) {
	return false, fake.refuse("IsRepositoryRoot")
}

func (fake *refusingGit) DefaultBranch(dir string) (string, error) {
	return "", fake.refuse("DefaultBranch")
}

func (fake *refusingGit) HasRemote(dir string, remote string) (bool, error) {
	return false, fake.refuse("HasRemote")
}

func (fake *refusingGit) Fetch(dir string, remote string, branch string) error {
	return fake.refuse("Fetch")
}

func (fake *refusingGit) AddDetachedWorktree(dir string, path string, ref string) error {
	return fake.refuse("AddDetachedWorktree")
}

func (fake *refusingGit) HasWorktree(dir string, path string) (bool, error) {
	return false, fake.refuse("HasWorktree")
}

func (fake *refusingGit) IsDirty(dir string) (bool, error) {
	return false, fake.refuse("IsDirty")
}

func (fake *refusingGit) Stash(dir string, message string) (string, error) {
	return "", fake.refuse("Stash")
}

func (fake *refusingGit) BranchExists(dir string, branch string) (bool, error) {
	return false, fake.refuse("BranchExists")
}

func (fake *refusingGit) CommitCount(dir string, base string, tip string) (int, error) {
	return 0, fake.refuse("CommitCount")
}

func (fake *refusingGit) CreateBranch(dir string, branch string, start string) error {
	return fake.refuse("CreateBranch")
}

func (fake *refusingGit) ResetBranch(dir string, branch string, start string) error {
	return fake.refuse("ResetBranch")
}

func (fake *refusingGit) DeleteBranch(dir string, branch string) error {
	return fake.refuse("DeleteBranch")
}

func (fake *refusingGit) CurrentBranch(dir string) (string, error) {
	return "", fake.refuse("CurrentBranch")
}

func (fake *refusingGit) BranchesContaining(dir string, commit string) ([]string, error) {
	return nil, fake.refuse("BranchesContaining")
}

func (fake *refusingGit) CheckoutDetached(dir string, ref string) error {
	return fake.refuse("CheckoutDetached")
}

func (fake *refusingGit) ResolveCommit(dir string, ref string) (git.Commit, error) {
	return git.Commit{}, fake.refuse("ResolveCommit")
}

func (fake *refusingGit) ResolveHeadCommit(dir string) (git.Commit, error) {
	return git.Commit{}, fake.refuse("ResolveHeadCommit")
}
