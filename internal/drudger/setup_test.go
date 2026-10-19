package drudger

import (
	"context"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/IgorBolotnikov/DRUDGE/internal/common"
	"github.com/IgorBolotnikov/DRUDGE/internal/project"
	"github.com/IgorBolotnikov/DRUDGE/internal/task"
)

// setupRepositories are a project whose repositories carry setup commands,
// all but the last one.
var setupRepositories = []project.Repository{
	{Path: "api", Setup: []string{"npm ci", "make deps"}},
	{Path: "ui", Setup: []string{"pnpm install"}},
	{Path: "docs"},
}

// settingsWithSetup are the settings of a project of the setup repositories.
func settingsWithSetup() Settings {
	settings := testSettings()
	settings.Repositories = setupRepositories
	return settings
}

// fakeExitError is a command that exited with a code.
type fakeExitError struct {
	code int
}

func (exitErr fakeExitError) Error() string {
	return fmt.Sprintf("exit status %d", exitErr.code)
}

func (exitErr fakeExitError) ExitCode() int {
	return exitErr.code
}

// setupCall is the sbx exec command line of one setup command.
func setupCall(projectDir string, repository string, command string) []string {
	worktree := filepath.Join(slotRoot(projectDir, 1), repository)
	return []string{sbxBinary, sbxExecSubcommand, sbxWorkdirFlag, worktree, testSandbox, shellBinary, shellCommandFlag, command}
}

// firstEventIndex returns where the first event of a type sits among the
// events a service reported, and -1 when there is none.
func firstEventIndex[Event any](progress *fakeProgress) int {
	return slices.IndexFunc(progress.events, func(event any) bool {
		_, ok := event.(Event)
		return ok
	})
}

func TestDrudgerService_RunTask_RunsTheSetupOfTheChangedRepositories(t *testing.T) {
	cases := []struct {
		name         string
		repositories []string
		// wantSetup is every setup command, as its repository and command.
		wantSetup [][2]string
	}{
		{
			name:      "a task naming no repositories sets up all of them in config order",
			wantSetup: [][2]string{{"api", "npm ci"}, {"api", "make deps"}, {"ui", "pnpm install"}},
		},
		{
			name:         "a task sets up only the repositories it lists",
			repositories: []string{"ui"},
			wantSetup:    [][2]string{{"ui", "pnpm install"}},
		},
		{
			name:         "a repository with no setup runs nothing",
			repositories: []string{"docs"},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			projectDir := setupProjectDir(t)
			taskToRun := todoTask()
			taskToRun.Repositories = testCase.repositories
			commands := &fakeCommandRunner{projectDir: projectDir, outputs: []string{sandboxListingWith()}}
			service := newTestServiceWith(settingsWithSetup(), commands, taskToRun)

			err := service.RunTask(testProjectSlug, taskToRun.ID, false)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			wantCalls := make([][]string, 0, len(testCase.wantSetup))
			for _, setup := range testCase.wantSetup {
				wantCalls = append(wantCalls, setupCall(projectDir, setup[0], setup[1]))
			}
			// The sandbox is listed and created first, and the agent start is the
			// last call.
			calls := commands.calls
			if got := commands.subcommands()[:2]; !slices.Equal(got, []string{sbxLsSubcommand, sbxCreateSubcommand}) {
				t.Fatalf("expected the sandbox to be listed and created first, got %v", commands.subcommands())
			}
			if got := calls[2 : len(calls)-1]; !slices.EqualFunc(got, wantCalls, slices.Equal[[]string]) {
				t.Errorf("expected the setup calls %q, got %q", wantCalls, got)
			}
			if got := calls[len(calls)-1]; !slices.Equal(got, commands.started[0]) {
				t.Errorf("expected the agent start to be the last call, got %q", got)
			}
			for index := range wantCalls {
				if got := commands.timeouts[2+index]; got != service.settings.SandboxTimeouts.Setup {
					t.Errorf("expected setup call %d to get %s, got %s", index, service.settings.SandboxTimeouts.Setup, got)
				}
			}

			started := reportedEvents[SetupCommandStarted](service.progress)
			if len(started) != len(testCase.wantSetup) {
				t.Fatalf("expected %d setup commands reported, got %+v", len(testCase.wantSetup), started)
			}
			for index, setup := range testCase.wantSetup {
				if started[index].Repository != setup[0] || started[index].Command != setup[1] {
					t.Errorf("expected setup %d to be %s of %s, got %+v", index, setup[1], setup[0], started[index])
				}
			}
			if taskToRun.Status != task.StatusInProgress {
				t.Errorf("expected status %q, got %q", task.StatusInProgress, taskToRun.Status)
			}
		})
	}
}

func TestDrudgerService_RunTask_RunsSetupBetweenTheSandboxAndTheAgent(t *testing.T) {
	projectDir := setupProjectDir(t)
	taskToRun := todoTask()
	commands := &fakeCommandRunner{projectDir: projectDir, outputs: []string{sandboxListingWith()}}
	service := newTestServiceWith(settingsWithSetup(), commands, taskToRun)

	err := service.RunTask(testProjectSlug, taskToRun.ID, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	order := []struct {
		step  string
		index int
	}{
		{step: "branch checkout", index: firstEventIndex[BranchCheckoutStarted](service.progress)},
		{step: "sandbox creation", index: firstEventIndex[SandboxCreationStarted](service.progress)},
		{step: "setup", index: firstEventIndex[SetupCommandStarted](service.progress)},
		{step: "agent start", index: firstEventIndex[AgentLaunchStarted](service.progress)},
	}
	for index, current := range order {
		if current.index < 0 {
			t.Fatalf("expected the %s to be reported, got %+v", current.step, service.progress.events)
		}
		if index > 0 && current.index < order[index-1].index {
			t.Errorf("expected the %s after the %s, got %+v", current.step, order[index-1].step, service.progress.events)
		}
	}
}

func TestDrudgerService_RunTask_WritesTheSetupLog(t *testing.T) {
	projectDir := setupProjectDir(t)
	taskToRun := todoTask()
	taskToRun.Repositories = []string{"api"}
	commands := &fakeCommandRunner{
		projectDir: projectDir,
		outputs:    []string{sandboxListingWith(), "", "added 120 packages", "deps ready"},
		stderrs:    []string{"", "", "npm warn deprecated", ""},
	}
	service := newTestServiceWith(settingsWithSetup(), commands, taskToRun)

	err := service.RunTask(testProjectSlug, taskToRun.ID, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	wantLog := "$ npm ci  # in api\nadded 120 packages\nnpm warn deprecated\n$ make deps  # in api\ndeps ready\n"
	if got, _ := service.runs.file(taskToRun.ID, common.RunSetupLogName); got != wantLog {
		t.Errorf("expected the setup log %q, got %q", wantLog, got)
	}

	var lines []string
	for _, output := range reportedEvents[SetupOutput](service.progress) {
		lines = append(lines, output.Line)
	}
	if want := []string{"added 120 packages", "npm warn deprecated", "deps ready"}; !slices.Equal(lines, want) {
		t.Errorf("expected the setup output %q, got %q", want, lines)
	}
}

func TestDrudgerService_RunTask_SetupFailureStopsTheLaunch(t *testing.T) {
	cases := []struct {
		name      string
		err       error
		wantInErr []string
	}{
		{
			name:      "a command that exits non-zero",
			err:       fakeExitError{code: 2},
			wantInErr: []string{"api", `"make deps"`, "exited with code 2"},
		},
		{
			name:      "a command killed by the timeout",
			err:       fmt.Errorf("command sbx did not finish within 15m0s and was killed: %w", context.DeadlineExceeded),
			wantInErr: []string{"api", `"make deps"`, SetupTimeoutKey},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			projectDir := setupProjectDir(t)
			taskToRun := todoTask()
			commands := &fakeCommandRunner{
				projectDir: projectDir,
				// The second setup command, make deps of api, fails.
				outputs: []string{sandboxListingWith(), "", "added 120 packages", "make: *** No rule to make target"},
				errs:    []error{nil, nil, nil, testCase.err},
			}
			service := newTestServiceWith(settingsWithSetup(), commands, taskToRun)

			err := service.RunTask(testProjectSlug, taskToRun.ID, false)

			if err == nil {
				t.Fatal("expected a failing setup command to stop the launch")
			}
			logPath := common.RunSetupLogPath(filepath.Join(projectDir, ".drudge", "runs", string(taskToRun.ID)))
			for _, want := range append(testCase.wantInErr, logPath) {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("expected the error to name %q, got %q", want, err)
				}
			}
			if got := len(commands.calls); got != 4 {
				t.Errorf("expected the setup to stop at the failing command, got calls %q", commands.calls)
			}
			if len(commands.started) != 0 {
				t.Errorf("expected no agent to start, got %q", commands.started)
			}
			if got, _ := service.runs.file(taskToRun.ID, common.RunSetupLogName); !strings.Contains(got, "No rule to make target") {
				t.Errorf("expected the setup log to hold the output of the failing command, got %q", got)
			}
			if taskToRun.Status != task.StatusTodo {
				t.Errorf("expected the task to stay %q, got %q", task.StatusTodo, taskToRun.Status)
			}
			if held := service.drudgers.holderOf(taskToRun.ID); held != nil {
				t.Errorf("expected the slot to be released, got Drudger %d holding the task", held.Slot)
			}
			recorded := service.drudgers.atSlot(1)
			if recorded.SandboxHealth != SandboxUsable || recorded.WorkspaceHealth != WorkspaceUsable || recorded.AgentHealth != "" {
				t.Errorf("expected the health records to be left alone, got sandbox %q, workspace %q, agent %q", recorded.SandboxHealth, recorded.WorkspaceHealth, recorded.AgentHealth)
			}
		})
	}
}

func TestDrudgerService_RunTask_DryRunListsTheSetupCommands(t *testing.T) {
	projectDir := setupProjectDir(t)
	taskToRun := todoTask()
	taskToRun.Repositories = []string{"ui", "docs"}
	commands := &fakeCommandRunner{projectDir: projectDir}
	service := newTestServiceWith(settingsWithSetup(), commands, taskToRun)

	if err := service.RunTask(testProjectSlug, taskToRun.ID, true); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	described := singleEvent[RunDescribed](t, service.progress)
	if got := len(described.Commands); got != 4 {
		t.Fatalf("expected the list, create, setup and start commands, got %q", described.Commands)
	}
	if got, want := described.Commands[2], setupCall(projectDir, "ui", "pnpm install"); !slices.Equal(got, want) {
		t.Errorf("expected the setup command %q, got %q", want, got)
	}
	if commands.calls != nil {
		t.Errorf("expected nothing to run, got %q", commands.calls)
	}
}
