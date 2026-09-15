package drudger

import (
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"drudge/internal/config"
	"drudge/internal/task"
)

// localConfigWith is the config of a project whose repositories sit at the
// given paths.
func localConfigWith(paths ...string) *config.LocalConfig {
	repositories := make([]config.Repository, 0, len(paths))
	for _, path := range paths {
		repositories = append(repositories, config.Repository{Path: path})
	}
	return &config.LocalConfig{ProjectSlug: testProjectSlug, Repositories: repositories}
}

// pathsIn prefixes relative paths with the directory they live in.
func pathsIn(dir string, paths ...string) []string {
	joined := make([]string, 0, len(paths))
	for _, path := range paths {
		joined = append(joined, filepath.Join(dir, path))
	}
	return joined
}

func TestDrudgerService_RunTask_CreatesAWorktreePerRepository(t *testing.T) {
	cases := []struct {
		name         string
		repositories []string
		// wantWorktrees is where each worktree goes, relative to the slot root.
		wantWorktrees []string
		// wantMounts are the repository mounts, relative to the project
		// directory. The slot root and the runs directory come with them.
		wantMounts []string
	}{
		{
			name:          "a project that is itself a repository",
			repositories:  []string{"."},
			wantWorktrees: []string{"."},
			wantMounts:    []string{".git"},
		},
		{
			name:          "a project of several repositories",
			repositories:  []string{"api", "ui"},
			wantWorktrees: []string{"api", "ui"},
			wantMounts:    []string{"api/.git", "ui/.git"},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			projectDir := setupProjectDir(t)
			taskToRun := todoTask()
			commands := &fakeCommandRunner{projectDir: projectDir, outputs: []string{sandboxListingWith()}}
			service := newTestServiceWith(localConfigWith(testCase.repositories...), config.DefaultConfig(), commands, taskToRun)

			var err error
			captureOutput(func() { err = service.RunTask(testProjectSlug, taskToRun.ID, false) })
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			root := slotRoot(projectDir, 1)
			wantWorktrees := pathsIn(root, testCase.wantWorktrees...)
			if got := service.git.worktreePaths(); !slices.Equal(got, wantWorktrees) {
				t.Errorf("expected worktrees %v, got %v", wantWorktrees, got)
			}

			wantCreate := []string{sbxBinary, sbxCreateSubcommand, sbxHarnessClaude, root}
			wantCreate = append(wantCreate, pathsIn(projectDir, testCase.wantMounts...)...)
			wantCreate = append(wantCreate, filepath.Join(projectDir, ".drudge", "runs"), sbxNameFlag, testSandbox)
			if got := commands.call(sbxCreateSubcommand); !slices.Equal(got, wantCreate) {
				t.Errorf("expected create %v, got %v", wantCreate, got)
			}

			start := commands.call(sbxExecSubcommand)
			if launcher := start[len(start)-1]; !strings.Contains(launcher, "cd '"+root+"' || exit 1") {
				t.Errorf("expected the agent to work in %s, got %q", root, launcher)
			}
			if recorded := service.drudgers.atSlot(1); recorded.Workspace != root {
				t.Errorf("expected the Drudger to record workspace %s, got %s", root, recorded.Workspace)
			}
		})
	}
}

func TestDrudgerService_RunTask_CutsAWorktreeFromTheDefaultBranch(t *testing.T) {
	cases := []struct {
		name        string
		noRemote    bool
		fetchErr    error
		wantFetched bool
		wantRef     string
	}{
		{name: "a repository with a remote is fetched first", wantFetched: true, wantRef: "origin/main"},
		{name: "a repository with no remote takes its local default branch", noRemote: true, wantRef: "main"},
		{name: "a fetch that fails leaves the run going", fetchErr: errors.New("could not reach origin"), wantFetched: true, wantRef: "origin/main"},
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
			captureOutput(func() { err = service.RunTask(testProjectSlug, taskToRun.ID, false) })
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			wantFetched := []string{projectDir}
			if !testCase.wantFetched {
				wantFetched = nil
			}
			if got := service.git.fetched; !slices.Equal(got, wantFetched) {
				t.Errorf("expected the fetches %v, got %v", wantFetched, got)
			}

			added := service.git.addedWorktrees
			if len(added) != 1 {
				t.Fatalf("expected one worktree, got %v", added)
			}
			if added[0].ref != testCase.wantRef {
				t.Errorf("expected the worktree cut from %q, got %q", testCase.wantRef, added[0].ref)
			}
			if taskToRun.Status != task.StatusInProgress {
				t.Errorf("expected status %q, got %q", task.StatusInProgress, taskToRun.Status)
			}
		})
	}
}

func TestDrudgerService_RunTask_ReusesTheWorkspaceOfTheSlot(t *testing.T) {
	projectDir := setupProjectDir(t)
	firstTask := todoTask()
	secondTask := todoTask()
	secondTask.ID = "task-2"

	commands := &fakeCommandRunner{
		projectDir: projectDir,
		// The second run lists the sandbox the first one created.
		outputs: []string{sandboxListingWith(), "", "", sandboxListingWith(testSandbox)},
	}
	service := newTestServiceWith(localConfigWith(testRepoPath), config.DefaultConfig(), commands, firstTask, secondTask)

	var err error
	captureOutput(func() { err = service.RunTask(testProjectSlug, firstTask.ID, false) })
	if err != nil {
		t.Fatalf("the first run failed: %v", err)
	}
	finishSession(t, projectDir, firstTask.ID)

	captureOutput(func() { err = service.RunTask(testProjectSlug, secondTask.ID, false) })
	if err != nil {
		t.Fatalf("the second run failed: %v", err)
	}

	if got := service.git.addedWorktrees; len(got) != 1 {
		t.Errorf("expected the second run to keep the worktree of the first, got %v", got)
	}
	if got := commands.callCount(sbxCreateSubcommand); got != 1 {
		t.Errorf("expected the sandbox to be created once, got %d creations", got)
	}
	if got := len(commands.started); got != 2 {
		t.Errorf("expected both agents to be started, got %d", got)
	}
}

func TestDrudgerService_RunTask_WorkspaceFailureLeavesTheTaskAlone(t *testing.T) {
	projectDir := setupProjectDir(t)
	taskToRun := todoTask()
	commands := &fakeCommandRunner{projectDir: projectDir, outputs: []string{sandboxListingWith()}}
	service := newTestServiceWith(localConfigWith("api", "ui"), config.DefaultConfig(), commands, taskToRun)
	service.git.worktreeErr = fmt.Errorf("fatal: invalid reference: origin/main")

	var err error
	captureOutput(func() { err = service.RunTask(testProjectSlug, taskToRun.ID, false) })

	if err == nil {
		t.Fatal("expected a workspace that could not be created to stop the run")
	}
	for _, want := range []string{"api", "invalid reference"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("expected the error to name %q, got %q", want, err)
		}
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
}

func TestDrudgerService_RunTask_RefusesAProjectWithNoRepositories(t *testing.T) {
	projectDir := setupProjectDir(t)
	taskToRun := todoTask()
	commands := &fakeCommandRunner{projectDir: projectDir, outputs: []string{sandboxListingWith()}}
	service := newTestServiceWith(localConfigWith(testRepoPath), config.DefaultConfig(), commands, taskToRun)
	// A project initialized before drudge recorded repositories has none.
	service.localCfg.Repositories = nil

	var err error
	captureOutput(func() { err = service.RunTask(testProjectSlug, taskToRun.ID, false) })

	if err == nil {
		t.Fatal("expected a project with no repositories to refuse the run")
	}
	if !strings.Contains(err.Error(), initCommand) {
		t.Errorf("expected the error to name %q, got %q", initCommand, err)
	}
	if len(service.git.addedWorktrees) != 0 {
		t.Errorf("expected no worktree, got %v", service.git.addedWorktrees)
	}
	if taskToRun.Status != task.StatusTodo {
		t.Errorf("expected the task to stay %q, got %q", task.StatusTodo, taskToRun.Status)
	}
}
