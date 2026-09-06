package drudger

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"drudge/internal/config"
	"drudge/internal/task"
)

// busyTask is the task a busy Drudger of a slot is working on.
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
		name  string
		pool  []*Drudger
		slot  int
		force bool
		// finished are the Sessions that have written their exit file before
		// the nuke.
		finished  []task.TaskID
		removeErr error
		// wantRemoved is the sandbox the removal command must name, empty when
		// nothing may be run at all.
		wantRemoved     string
		wantSlotsLeft   []int
		wantErrContains string
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
			force:          true,
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
			wantRemoved:     testSandboxOfSlot(1),
			wantSlotsLeft:   []int{1},
			wantErrContains: removalFailed.Error(),
		},
		{
			name:            "a failed removal of a forced Drudger leaves its task alone",
			pool:            []*Drudger{busyDrudger(1)},
			slot:            1,
			force:           true,
			removeErr:       removalFailed,
			wantRemoved:     testSandboxOfSlot(1),
			wantSlotsLeft:   []int{1},
			wantErrContains: removalFailed.Error(),
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			workspace := setupWorkspace(t)
			for _, finishedID := range testCase.finished {
				finishSession(t, workspace, finishedID)
			}

			commands := &fakeCommandRunner{workspace: workspace, errs: []error{testCase.removeErr}}
			occupied := busyTask(testCase.slot)
			service := newTestServiceWithPool(
				&config.LocalConfig{ProjectSlug: testProjectSlug},
				config.DefaultConfig(),
				commands,
				testCase.pool,
				occupied,
			)

			var err error
			captureOutput(func() { err = service.NukeDrudger(testProjectSlug, testCase.slot, testCase.force) })

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
	setupWorkspace(t)
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

func slotsOf(drudgers []*Drudger) []int {
	slots := make([]int, 0, len(drudgers))
	for _, candidate := range drudgers {
		slots = append(slots, candidate.Slot)
	}
	slices.Sort(slots)
	return slots
}
