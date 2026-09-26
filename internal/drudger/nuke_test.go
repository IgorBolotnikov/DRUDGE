package drudger

import (
	"errors"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/IgorBolotnikov/DRUDGE/internal/common"
	"github.com/IgorBolotnikov/DRUDGE/internal/config"
	"github.com/IgorBolotnikov/DRUDGE/internal/task"
)

func busyTask(slot int) *task.Task {
	return &task.Task{
		ID:          busyTaskID(slot),
		Title:       "Fix login",
		Status:      task.StatusInProgress,
		ProjectSlug: testProjectSlug,
	}
}

func TestDrudgerService_NukeDrudger(t *testing.T) {
	removalFailed := errors.New("sbx said no")

	cases := []struct {
		name     string
		pool     []*Drudger
		slot     int
		isForced bool
		// finished are the Sessions that have written their exit file before
		// the nuke.
		finished  []task.TaskID
		removeErr error
		// listing is what the sandbox listing answers after a failed removal.
		listing string
		// wantRemoved is the sandbox the removal command must name, empty when
		// nothing may be run at all.
		wantRemoved     string
		wantListed      bool
		wantSlotsLeft   []int
		wantErrContains string
		// taskStatus is what the task of the nuked Drudger is recorded as
		// before the nuke. An empty value stands for a task still running.
		taskStatus task.TaskStatus
		// wantTaskStatus is what the task of the nuked Drudger ends up as. An
		// empty value expects it to be left alone.
		wantTaskStatus task.TaskStatus
	}{
		{
			name:          "an idle Drudger goes without ceremony",
			pool:          []*Drudger{idleDrudger(1)},
			slot:          1,
			wantRemoved:   testSandboxOfSlot(1),
			wantSlotsLeft: []int{},
		},
		{
			name:          "only the named slot leaves the pool",
			pool:          []*Drudger{idleDrudger(1), idleDrudger(2), idleDrudger(3)},
			slot:          2,
			wantRemoved:   testSandboxOfSlot(2),
			wantSlotsLeft: []int{1, 3},
		},
		{
			name:            "a working Drudger is refused",
			pool:            []*Drudger{busyDrudger(1)},
			slot:            1,
			wantSlotsLeft:   []int{1},
			wantErrContains: string(busyTaskID(1)),
		},
		{
			name:           "forcing a working Drudger kills its agent",
			pool:           []*Drudger{busyDrudger(1)},
			slot:           1,
			isForced:       true,
			wantRemoved:    testSandboxOfSlot(1),
			wantSlotsLeft:  []int{},
			wantTaskStatus: task.StatusFuckedUp,
		},
		{
			name:          "a Drudger whose Session has finished needs no force",
			pool:          []*Drudger{busyDrudger(1)},
			slot:          1,
			finished:      []task.TaskID{busyTaskID(1)},
			wantRemoved:   testSandboxOfSlot(1),
			wantSlotsLeft: []int{},
		},
		{
			name:            "a slot holding no Drudger",
			pool:            []*Drudger{idleDrudger(1)},
			slot:            2,
			wantSlotsLeft:   []int{1},
			wantErrContains: "no Drudger in slot 2",
		},
		{
			name:            "an empty pool",
			pool:            nil,
			slot:            1,
			wantSlotsLeft:   []int{},
			wantErrContains: "no Drudger in slot 1",
		},
		{
			name:            "a failed removal keeps the entry",
			pool:            []*Drudger{idleDrudger(1)},
			slot:            1,
			removeErr:       removalFailed,
			listing:         sandboxListingWith(testSandboxOfSlot(1)),
			wantRemoved:     testSandboxOfSlot(1),
			wantListed:      true,
			wantSlotsLeft:   []int{1},
			wantErrContains: removalFailed.Error(),
		},
		{
			name:          "a sandbox that is already gone counts as removed",
			pool:          []*Drudger{idleDrudger(1)},
			slot:          1,
			removeErr:     removalFailed,
			listing:       sandboxListingWith(testSandboxOfSlot(2)),
			wantRemoved:   testSandboxOfSlot(1),
			wantListed:    true,
			wantSlotsLeft: []int{},
		},
		{
			name:           "forcing a Drudger whose sandbox is already gone kills its task",
			pool:           []*Drudger{busyDrudger(1)},
			slot:           1,
			isForced:       true,
			removeErr:      removalFailed,
			listing:        sandboxListingWith(),
			wantRemoved:    testSandboxOfSlot(1),
			wantListed:     true,
			wantSlotsLeft:  []int{},
			wantTaskStatus: task.StatusFuckedUp,
		},
		{
			name:            "a failed removal that cannot be checked keeps the entry",
			pool:            []*Drudger{idleDrudger(1)},
			slot:            1,
			removeErr:       removalFailed,
			listing:         "not json",
			wantRemoved:     testSandboxOfSlot(1),
			wantListed:      true,
			wantSlotsLeft:   []int{1},
			wantErrContains: removalFailed.Error(),
		},
		{
			name:           "forcing a Drudger whose task moved on leaves the task alone",
			pool:           []*Drudger{busyDrudger(1)},
			slot:           1,
			isForced:       true,
			taskStatus:     task.StatusTodo,
			wantRemoved:    testSandboxOfSlot(1),
			wantSlotsLeft:  []int{},
			wantTaskStatus: task.StatusTodo,
		},
		{
			name:            "a failed removal of a forced Drudger leaves its task alone",
			pool:            []*Drudger{busyDrudger(1)},
			slot:            1,
			isForced:        true,
			removeErr:       removalFailed,
			listing:         sandboxListingWith(testSandboxOfSlot(1)),
			wantRemoved:     testSandboxOfSlot(1),
			wantListed:      true,
			wantSlotsLeft:   []int{1},
			wantErrContains: removalFailed.Error(),
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			projectDir := setupProjectDir(t)
			for _, finishedID := range testCase.finished {
				finishSession(t, projectDir, finishedID)
			}

			commands := &fakeCommandRunner{
				projectDir: projectDir,
				outputs:    []string{"", testCase.listing},
				errs:       []error{testCase.removeErr},
			}
			occupied := busyTask(testCase.slot)
			if testCase.taskStatus != "" {
				occupied.Status = testCase.taskStatus
			}
			service := newTestServiceWithPool(
				&config.LocalConfig{ProjectSlug: testProjectSlug},
				config.DefaultConfig(),
				commands,
				testCase.pool,
				occupied,
			)

			var err error
			captureOutput(func() { err = service.NukeDrudger(testProjectSlug, testCase.slot, testCase.isForced) })

			if testCase.wantErrContains == "" && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if testCase.wantErrContains != "" {
				if err == nil {
					t.Fatalf("expected an error naming %q", testCase.wantErrContains)
				}
				if !strings.Contains(err.Error(), testCase.wantErrContains) {
					t.Errorf("expected error to name %q, got %q", testCase.wantErrContains, err)
				}
			}

			wantCalls := [][]string{}
			if testCase.wantRemoved != "" {
				wantCalls = append(wantCalls, []string{sbxBinary, sbxRmSubcommand, sbxForceFlag, testCase.wantRemoved})
			}
			if testCase.wantListed {
				wantCalls = append(wantCalls, []string{sbxBinary, sbxLsSubcommand, sbxJSONFlag})
			}
			if !slices.EqualFunc(commands.calls, wantCalls, slices.Equal) {
				t.Errorf("expected commands %v, got %v", wantCalls, commands.calls)
			}

			if got := slotsOf(service.drudgers.drudgers); !slices.Equal(got, testCase.wantSlotsLeft) {
				t.Errorf("expected slots %v to be left, got %v", testCase.wantSlotsLeft, got)
			}

			wantStatus := testCase.wantTaskStatus
			if wantStatus == "" {
				wantStatus = task.StatusInProgress
			}
			if occupied.Status != wantStatus {
				t.Errorf("expected task %s to be %q, got %q", occupied.ID, wantStatus, occupied.Status)
			}
			if wantStatus == task.StatusFuckedUp && occupied.FinishedAt.IsZero() {
				t.Error("expected finished at to be stamped on the killed task")
			}
		})
	}
}

func TestDrudgerService_NukeDrudger_UnsupportedEnvironment(t *testing.T) {
	setupProjectDir(t)
	commands := &fakeCommandRunner{}
	service := newTestServiceWithPool(
		&config.LocalConfig{ProjectSlug: testProjectSlug},
		&config.GlobalConfig{Drudger: config.DrudgerConfig{Env: config.Env("bare-metal")}},
		commands,
		[]*Drudger{idleDrudger(1)},
	)

	var err error
	captureOutput(func() { err = service.NukeDrudger(testProjectSlug, 1, false) })
	if err == nil {
		t.Fatal("expected an error naming the environment")
	}
	if !strings.Contains(err.Error(), "bare-metal") {
		t.Errorf("expected error to name the environment, got %q", err)
	}
	if commands.calls != nil {
		t.Errorf("expected nothing to be run, got %v", commands.calls)
	}
	if service.drudgers.atSlot(1) == nil {
		t.Error("expected the Drudger to stay in the pool")
	}
}

func TestDrudgerService_NukeDrudger_TakesTheWorkspace(t *testing.T) {
	cases := []struct {
		name string
		// An empty list stands for the single repository most cases work with.
		repositories []string
		// leave puts the worktrees in the state the nuke finds them in, keyed
		// by repository name.
		leave func(fake *fakeGit, worktrees map[string]string)
		// absent are the repositories whose worktree directory is gone from
		// disk before the nuke runs.
		absent []string

		// wantRemovedIn names the repositories whose worktree the nuke
		// deletes, and wantStashedIn the ones it stashes first.
		wantRemovedIn []string
		wantStashedIn []string
	}{
		{
			name:          "a clean workspace goes with no stash",
			wantRemovedIn: []string{testRepositoryName},
		},
		{
			name: "a dirty worktree is stashed before it goes",
			leave: func(fake *fakeGit, worktrees map[string]string) {
				fake.leaveDirty(worktrees[testRepositoryName])
			},
			wantRemovedIn: []string{testRepositoryName},
			wantStashedIn: []string{testRepositoryName},
		},
		{
			name:         "every repository of a workspace goes",
			repositories: []string{"api", "ui"},
			leave: func(fake *fakeGit, worktrees map[string]string) {
				fake.leaveDirty(worktrees["ui"])
			},
			wantRemovedIn: []string{"api", "ui"},
			wantStashedIn: []string{"ui"},
		},
		{
			name:   "a worktree that is already gone is pruned all the same",
			absent: []string{testRepositoryName},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			repositories := testCase.repositories
			if len(repositories) == 0 {
				repositories = []string{testRepositoryName}
			}

			projectDir := setupProjectDir(t)
			doomed := handedOverTask(repositories...)
			pool := []*Drudger{holdingDrudger(projectDir, doomed.ID)}
			commands := &fakeCommandRunner{projectDir: projectDir}
			service := newTestServiceWithPool(localConfigWith(repositories...), config.DefaultConfig(), commands, pool, doomed)

			worktrees := worktreesOf(projectDir, repositories)
			makeWorktrees(t, worktrees)
			for _, repository := range testCase.absent {
				if err := os.RemoveAll(worktrees[repository]); err != nil {
					t.Fatalf("could not delete the worktree %s: %v", worktrees[repository], err)
				}
			}
			if testCase.leave != nil {
				testCase.leave(service.git, worktrees)
			}

			var err error
			captureOutput(func() { err = service.NukeDrudger(testProjectSlug, 1, true) })
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			wantRemoved := pathsOf(worktrees, testCase.wantRemovedIn)
			if !slices.Equal(service.git.removedWorktrees, wantRemoved) {
				t.Errorf("expected the worktrees %v to be removed, got %v", wantRemoved, service.git.removedWorktrees)
			}

			wantPruned := pathsIn(projectDir, repositories...)
			if !slices.Equal(service.git.prunedRepositories, wantPruned) {
				t.Errorf("expected the repositories %v to be pruned, got %v", wantPruned, service.git.prunedRepositories)
			}

			wantStashed := pathsOf(worktrees, testCase.wantStashedIn)
			if got := stashedDirs(service.git.stashes); !slices.Equal(got, wantStashed) {
				t.Errorf("expected the worktrees %v to be stashed, got %v", wantStashed, got)
			}
			for index, repository := range testCase.wantStashedIn {
				want := service.git.stashes[index].sha
				if got := doomed.Stashes[repository]; got != want {
					t.Errorf("expected repository %s to record stash %s, got %q", repository, want, got)
				}
			}

			for _, worktree := range worktrees {
				isPresent, err := common.Exists(worktree)
				if err != nil {
					t.Fatalf("could not read %s: %v", worktree, err)
				}
				if isPresent {
					t.Errorf("expected the worktree %s to be gone", worktree)
				}
			}
			if service.drudgers.atSlot(1) != nil {
				t.Error("expected the Drudger to leave the pool")
			}
		})
	}
}

func TestDrudgerService_NukeDrudger_TakesTheWorkspaceBeforeTheSandbox(t *testing.T) {
	projectDir := setupProjectDir(t)
	commands := &fakeCommandRunner{projectDir: projectDir, errs: []error{errors.New("sbx said no")}}
	pool := []*Drudger{idleDrudgerAt(projectDir, 1)}
	service := newTestServiceWithPool(localConfigWith(testRepositoryName), config.DefaultConfig(), commands, pool)
	makeWorktrees(t, worktreesOf(projectDir, []string{testRepositoryName}))

	var err error
	captureOutput(func() { err = service.NukeDrudger(testProjectSlug, 1, false) })
	if err == nil {
		t.Fatal("expected the failed sandbox removal to be reported")
	}

	wantRemoved := []string{slotWorktree(projectDir, 1)}
	if !slices.Equal(service.git.removedWorktrees, wantRemoved) {
		t.Errorf("expected the worktree to go before the sandbox, got %v", service.git.removedWorktrees)
	}
}

func TestDrudgerService_NukeDrudger_KeepsTheWorkspaceOfALiveSession(t *testing.T) {
	projectDir := setupProjectDir(t)
	tracked := handedOverTask(testRepositoryName)
	writeStream(t, common.RunDir(projectDir, string(tracked.ID)), initEvent, assistantEvent)

	commands := &fakeCommandRunner{projectDir: projectDir}
	pool := []*Drudger{holdingDrudger(projectDir, tracked.ID)}
	service := newTestServiceWithPool(localConfigWith(testRepositoryName), config.DefaultConfig(), commands, pool, tracked)
	service.gitOps = &refusingGit{t: t}

	var err error
	captureOutput(func() { err = service.NukeDrudger(testProjectSlug, 1, false) })
	if err == nil {
		t.Fatal("expected a working Drudger to be refused")
	}
	if commands.calls != nil {
		t.Errorf("expected nothing to be run, got %v", commands.calls)
	}
}

func TestDrudgerService_NukeDrudger_RunsNoGitWithoutAWorkspace(t *testing.T) {
	projectDir := setupProjectDir(t)
	commands := &fakeCommandRunner{projectDir: projectDir}
	service := newTestServiceWithPool(localConfigWith(testRepositoryName), config.DefaultConfig(), commands, []*Drudger{idleDrudger(1)})
	service.gitOps = &refusingGit{t: t}

	var err error
	captureOutput(func() { err = service.NukeDrudger(testProjectSlug, 1, false) })
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if service.drudgers.atSlot(1) != nil {
		t.Error("expected the Drudger to leave the pool")
	}
}

func TestDrudgerService_NukeDrudger_ReportsAWorkspaceItCannotTakeApart(t *testing.T) {
	projectDir := setupProjectDir(t)
	commands := &fakeCommandRunner{projectDir: projectDir}
	pool := []*Drudger{idleDrudgerAt(projectDir, 1)}
	service := newTestServiceWithPool(localConfigWith(testRepositoryName), config.DefaultConfig(), commands, pool)
	service.git.removalErr = errors.New("git said no")
	makeWorktrees(t, worktreesOf(projectDir, []string{testRepositoryName}))

	var err error
	reported := captureErrors(func() { err = service.NukeDrudger(testProjectSlug, 1, false) })
	if err != nil {
		t.Fatalf("expected the nuke to go through, got %v", err)
	}
	if !strings.Contains(reported, "git said no") {
		t.Errorf("expected the failure to be reported, got %q", reported)
	}
	if service.drudgers.atSlot(1) != nil {
		t.Error("expected the Drudger to leave the pool")
	}
}

func slotsOf(drudgers []*Drudger) []int {
	slots := make([]int, 0, len(drudgers))
	for _, candidate := range drudgers {
		slots = append(slots, candidate.Slot)
	}
	slices.Sort(slots)
	return slots
}
