package drudger

import (
	"errors"
	"maps"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"drudge/internal/config"
	"drudge/internal/git"
	"drudge/internal/task"
)

// testTaskBranch is the branch the task todoTask returns gets.
const testTaskBranch = "drudge/task-1-fix-login"

func TestDrudgerService_RunTask_StashesOnlyADirtyWorktree(t *testing.T) {
	cases := []struct {
		name      string
		dirty     bool
		wantStash bool
	}{
		{name: "a clean worktree is left alone", dirty: false},
		{name: "a dirty worktree is put aside", dirty: true, wantStash: true},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			projectDir := setupProjectDir(t)
			taskToRun := todoTask()
			commands := &fakeCommandRunner{projectDir: projectDir, outputs: []string{sandboxListingWith()}}
			service := newTestServiceWith(localConfigWith(testRepoPath), config.DefaultConfig(), commands, taskToRun)
			worktree := slotRoot(projectDir, 1)
			if testCase.dirty {
				service.git.dirtyWorktrees = map[string]bool{worktree: true}
			}

			var err error
			captureOutput(func() { err = service.RunTask(testProjectSlug, taskToRun.ID, false) })
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			stashes := service.git.stashes
			if !testCase.wantStash {
				if len(stashes) != 0 {
					t.Fatalf("expected a clean worktree not to be stashed, got %v", stashes)
				}
				if len(taskToRun.Stashes) != 0 {
					t.Errorf("expected the task to record no stash, got %v", taskToRun.Stashes)
				}
				return
			}

			if len(stashes) != 1 {
				t.Fatalf("expected one stash, got %v", stashes)
			}
			if stashes[0].dir != worktree {
				t.Errorf("expected the stash to be made in %s, got %s", worktree, stashes[0].dir)
			}
			for _, want := range []string{"slot 1", string(taskToRun.ID)} {
				if !strings.Contains(stashes[0].message, want) {
					t.Errorf("expected the stash message to name %q, got %q", want, stashes[0].message)
				}
			}
			if got := taskToRun.Stashes[filepath.Base(projectDir)]; got != stashes[0].sha {
				t.Errorf("expected the task to record stash %s, got %q", stashes[0].sha, got)
			}
		})
	}
}

func TestDrudgerService_RunTask_CutsTheTaskBranchFromTheDefault(t *testing.T) {
	cases := []struct {
		name        string
		noRemote    bool
		fetchErr    error
		wantFetches int
		wantStart   string
		wantWarning bool
	}{
		{name: "a repository with a remote branches off the fetched default", wantFetches: 1, wantStart: "origin/main"},
		{name: "a repository with no remote branches off the local default", noRemote: true, wantStart: "main"},
		{name: "a fetch that fails leaves the run going", fetchErr: errors.New("could not reach origin"), wantFetches: 1, wantStart: "origin/main", wantWarning: true},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			projectDir := setupProjectDir(t)
			taskToRun := todoTask()
			commands := &fakeCommandRunner{projectDir: projectDir, outputs: []string{sandboxListingWith()}}
			service := newTestServiceWith(localConfigWith(testRepoPath), config.DefaultConfig(), commands, taskToRun)
			service.git.noRemote = testCase.noRemote
			service.git.fetchErr = testCase.fetchErr

			var err error
			warnings := captureErrors(func() {
				captureOutput(func() { err = service.RunTask(testProjectSlug, taskToRun.ID, false) })
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if got := len(service.git.fetched); got != testCase.wantFetches {
				t.Errorf("expected %d fetches, got %v", testCase.wantFetches, service.git.fetched)
			}

			created := service.git.createdBranches
			if len(created) != 1 {
				t.Fatalf("expected one branch, got %v", created)
			}
			if created[0].branch != testTaskBranch {
				t.Errorf("expected branch %q, got %q", testTaskBranch, created[0].branch)
			}
			if created[0].start != testCase.wantStart {
				t.Errorf("expected the branch cut from %q, got %q", testCase.wantStart, created[0].start)
			}
			if worktree := slotRoot(projectDir, 1); created[0].dir != worktree {
				t.Errorf("expected the branch to be put on %s, got %s", worktree, created[0].dir)
			}
			if taskToRun.Status != task.StatusInProgress {
				t.Errorf("expected status %q, got %q", task.StatusInProgress, taskToRun.Status)
			}

			if !testCase.wantWarning {
				return
			}
			for _, want := range []string{git.ShortSHA(testBaseSHA), testCase.wantStart} {
				if !strings.Contains(warnings, want) {
					t.Errorf("expected the warning to name %q, got %q", want, warnings)
				}
			}
		})
	}
}

func TestDrudgerService_RunTask_NamesTheBranchAfterTheTask(t *testing.T) {
	cases := []struct {
		name  string
		id    task.TaskID
		title string
		want  string
	}{
		{name: "the short id and the title", id: "task-1", title: "Fix login", want: "drudge/task-1-fix-login"},
		{name: "a full uuid is cut to its leading characters", id: "a1b2c3d4-e5f6-7890-abcd-ef1234567890", title: "Fix login", want: "drudge/a1b2c3d4-fix-login"},
		{name: "punctuation a branch cannot carry is dropped", id: "task-1", title: "Fix: login/SSO (again)?", want: "drudge/task-1-fix-login-sso-again"},
		{name: "a title of nothing usable leaves the id alone", id: "task-1", title: "!?", want: "drudge/task-1"},
		// Thirteen words are the most that fit under branchSlugLength.
		{name: "a long title keeps the words that fit", id: "task-1", title: strings.Repeat("ab ", 30), want: "drudge/task-1-" + strings.TrimSuffix(strings.Repeat("ab-", 13), "-")},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			projectDir := setupProjectDir(t)
			taskToRun := todoTask()
			taskToRun.ID = testCase.id
			taskToRun.Title = testCase.title
			commands := &fakeCommandRunner{projectDir: projectDir, outputs: []string{sandboxListingWith()}}
			service := newTestServiceWith(localConfigWith(testRepoPath), config.DefaultConfig(), commands, taskToRun)

			var err error
			captureOutput(func() { err = service.RunTask(testProjectSlug, taskToRun.ID, false) })
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if got := branchesOf(service.git.createdBranches); !slices.Equal(got, []string{testCase.want}) {
				t.Errorf("expected branch %q, got %v", testCase.want, got)
			}
		})
	}
}

func TestDrudgerService_RerunTask_ReusesABranchThatHoldsNoWork(t *testing.T) {
	cases := []struct {
		name string
		// existing is how many commits each branch already there holds beyond
		// the default branch.
		existing   map[string]int
		wantBranch string
		wantReused bool
	}{
		{
			name:       "a first attempt creates the branch",
			wantBranch: testTaskBranch,
		},
		{
			name:       "a branch holding nothing is reset and reused",
			existing:   map[string]int{testTaskBranch: 0},
			wantBranch: testTaskBranch,
			wantReused: true,
		},
		{
			name:       "a branch holding commits keeps them and the rerun takes the next name",
			existing:   map[string]int{testTaskBranch: 2},
			wantBranch: testTaskBranch + "-2",
		},
		{
			name:       "two attempts holding commits take the third name",
			existing:   map[string]int{testTaskBranch: 2, testTaskBranch + "-2": 1},
			wantBranch: testTaskBranch + "-3",
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			projectDir := setupProjectDir(t)
			taskToRerun := todoTask()
			taskToRerun.Status = task.StatusFuckedUp
			commands := &fakeCommandRunner{projectDir: projectDir, outputs: []string{sandboxListingWith()}}
			service := newTestServiceWith(localConfigWith(testRepoPath), config.DefaultConfig(), commands, taskToRerun)
			service.git.branchCommits = maps.Clone(testCase.existing)

			var err error
			captureOutput(func() { err = service.RerunTask(testProjectSlug, taskToRerun.ID, false) })
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			put := service.git.createdBranches
			if testCase.wantReused {
				put = service.git.resetBranches
			}
			if got := branchesOf(put); !slices.Equal(got, []string{testCase.wantBranch}) {
				t.Errorf("expected branch %q, got %v", testCase.wantBranch, got)
			}

			moved := append(branchesOf(service.git.createdBranches), branchesOf(service.git.resetBranches)...)
			for branch, commits := range testCase.existing {
				if commits > 0 && slices.Contains(moved, branch) {
					t.Errorf("expected branch %s holding %d commits to be left where it is", branch, commits)
				}
			}
		})
	}
}

func TestDrudgerService_RunTask_HousekeepingFailureStopsTheRun(t *testing.T) {
	cases := []struct {
		name      string
		dirty     bool
		stashErr  error
		branchErr error
		wantInErr string
	}{
		{
			name:      "a stash that fails",
			dirty:     true,
			stashErr:  errors.New("git stash refused"),
			wantInErr: "stash",
		},
		{
			name:      "a branch that cannot be created",
			branchErr: errors.New("git switch refused"),
			wantInErr: "branch",
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			projectDir := setupProjectDir(t)
			taskToRun := todoTask()
			commands := &fakeCommandRunner{projectDir: projectDir, outputs: []string{sandboxListingWith()}}
			service := newTestServiceWith(localConfigWith(testRepoPath), config.DefaultConfig(), commands, taskToRun)
			service.git.stashErr = testCase.stashErr
			service.git.branchErr = testCase.branchErr
			if testCase.dirty {
				service.git.dirtyWorktrees = map[string]bool{slotRoot(projectDir, 1): true}
			}

			var err error
			captureOutput(func() { err = service.RunTask(testProjectSlug, taskToRun.ID, false) })

			if err == nil {
				t.Fatal("expected housekeeping that failed to stop the run")
			}
			if !strings.Contains(err.Error(), testCase.wantInErr) {
				t.Errorf("expected the error to name the step %q, got %q", testCase.wantInErr, err)
			}
			if commands.calls != nil {
				t.Errorf("expected no sandbox command to run, got %v", commands.subcommands())
			}
			if taskToRun.Status != task.StatusTodo {
				t.Errorf("expected the task to stay %q, got %q", task.StatusTodo, taskToRun.Status)
			}
			if held := service.drudgers.holderOf(taskToRun.ID); held != nil {
				t.Errorf("expected the slot to be released, got Drudger %d holding the task", held.Slot)
			}
		})
	}
}

func TestDrudgerService_RunTask_RunsHousekeepingInEveryRepository(t *testing.T) {
	repositories := []string{"api", "ui"}

	projectDir := setupProjectDir(t)
	taskToRun := todoTask()
	commands := &fakeCommandRunner{projectDir: projectDir, outputs: []string{sandboxListingWith()}}
	service := newTestServiceWith(localConfigWith(repositories...), config.DefaultConfig(), commands, taskToRun)
	worktrees := pathsIn(slotRoot(projectDir, 1), repositories...)
	service.git.dirtyWorktrees = map[string]bool{worktrees[0]: true, worktrees[1]: true}

	var err error
	captureOutput(func() { err = service.RunTask(testProjectSlug, taskToRun.ID, false) })
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := len(service.git.fetched); got != len(repositories) {
		t.Errorf("expected every repository to be fetched, got %v", service.git.fetched)
	}

	if stashed := stashedDirs(service.git.stashes); !slices.Equal(stashed, worktrees) {
		t.Errorf("expected every worktree to be stashed, got %v", stashed)
	}

	wantBranches := []string{testTaskBranch, testTaskBranch}
	if got := branchesOf(service.git.createdBranches); !slices.Equal(got, wantBranches) {
		t.Errorf("expected the same branch in every repository, got %v", got)
	}

	for index, repository := range repositories {
		if got, want := taskToRun.Stashes[repository], service.git.stashes[index].sha; got != want {
			t.Errorf("expected repository %s to record stash %s, got %q", repository, want, got)
		}
	}
}

func TestDrudgerService_RunTask_RecordsWhereTheWorkWillBe(t *testing.T) {
	repositories := []string{"api", "ui"}

	projectDir := setupProjectDir(t)
	taskToRun := todoTask()
	commands := &fakeCommandRunner{projectDir: projectDir, outputs: []string{sandboxListingWith()}}
	service := newTestServiceWith(localConfigWith(repositories...), config.DefaultConfig(), commands, taskToRun)

	var err error
	captureOutput(func() { err = service.RunTask(testProjectSlug, taskToRun.ID, false) })
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := map[string]task.Landing{
		"api": {Branch: testTaskBranch, Base: testBaseSHA},
		"ui":  {Branch: testTaskBranch, Base: testBaseSHA},
	}
	if !maps.Equal(taskToRun.Landings, want) {
		t.Errorf("expected the handover to record %v, got %v", want, taskToRun.Landings)
	}
}
