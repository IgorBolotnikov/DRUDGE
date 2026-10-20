package drudger

import (
	"context"
	"fmt"
	"os"
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

// setupCall is the sbx exec command line of one setup command of a
// repository whose hooks directory under the sandbox home is hooksDir.
func setupCall(projectDir string, repository string, hooksDir string, command string) []string {
	worktree := filepath.Join(slotRoot(projectDir, 1), repository)
	script := formatPathPreamble() + `export DRUDGE_HOOKS="$HOME"/'` + hooksDir + "'\n" + command
	return []string{sbxBinary, sbxExecSubcommand, sbxWorkdirFlag, worktree, testSandbox, shellBinary, shellCommandFlag, script}
}

// resetCall is the sbx exec command line that empties the path file and
// creates the given hooks directories under the sandbox home.
func resetCall(hooksDirs ...string) []string {
	script := `mkdir -p "$HOME"/'.drudge'`
	for _, hooksDir := range hooksDirs {
		script += ` "$HOME"/'` + hooksDir + `'`
	}
	script += ` && : > "$HOME"/'.drudge/path'`
	return []string{sbxBinary, sbxExecSubcommand, testSandbox, shellBinary, shellCommandFlag, script}
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
		// wantHooks are the hooks directories the reset creates.
		wantHooks []string
		// wantSetup is every setup command, as its repository and command.
		wantSetup [][2]string
	}{
		{
			name:      "a task naming no repositories sets up all of them in config order",
			wantHooks: []string{".drudge/hooks/api", ".drudge/hooks/ui"},
			wantSetup: [][2]string{{"api", "npm ci"}, {"api", "make deps"}, {"ui", "pnpm install"}},
		},
		{
			name:         "a task sets up only the repositories it lists",
			repositories: []string{"ui"},
			wantHooks:    []string{".drudge/hooks/ui"},
			wantSetup:    [][2]string{{"ui", "pnpm install"}},
		},
		{
			name:         "a repository with no setup only empties the path file",
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

			wantCalls := [][]string{resetCall(testCase.wantHooks...)}
			for _, setup := range testCase.wantSetup {
				wantCalls = append(wantCalls, setupCall(projectDir, setup[0], ".drudge/hooks/"+setup[0], setup[1]))
			}
			// The sandbox is listed and created first, the reset runs before the
			// setup and the agent start is the last call.
			calls := commands.calls
			if got := commands.subcommands()[:2]; !slices.Equal(got, []string{sbxLsSubcommand, sbxCreateSubcommand}) {
				t.Fatalf("expected the sandbox to be listed and created first, got %v", commands.subcommands())
			}
			if got := calls[2 : len(calls)-1]; !slices.EqualFunc(got, wantCalls, slices.Equal[[]string]) {
				t.Errorf("expected the reset and setup calls %q, got %q", wantCalls, got)
			}
			if got := calls[len(calls)-1]; !slices.Equal(got, commands.started[0]) {
				t.Errorf("expected the agent start to be the last call, got %q", got)
			}
			for index := range wantCalls {
				if got := commands.timeouts[2+index]; got != service.settings.SandboxTimeouts.Setup {
					t.Errorf("expected call %d after the create to get %s, got %s", index, service.settings.SandboxTimeouts.Setup, got)
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
		outputs:    []string{sandboxListingWith(), "", "", "added 120 packages", "deps ready"},
		stderrs:    []string{"", "", "", "npm warn deprecated", ""},
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
				outputs: []string{sandboxListingWith(), "", "", "added 120 packages", "make: *** No rule to make target"},
				errs:    []error{nil, nil, nil, nil, testCase.err},
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
			if got := len(commands.calls); got != 5 {
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
	if got := len(described.Commands); got != 5 {
		t.Fatalf("expected the list, create, reset, setup and start commands, got %q", described.Commands)
	}
	if got, want := described.Commands[2], resetCall(".drudge/hooks/ui"); !slices.Equal(got, want) {
		t.Errorf("expected the reset command %q, got %q", want, got)
	}
	if got, want := described.Commands[3], setupCall(projectDir, "ui", ".drudge/hooks/ui", "pnpm install"); !slices.Equal(got, want) {
		t.Errorf("expected the setup command %q, got %q", want, got)
	}
	if commands.calls != nil {
		t.Errorf("expected nothing to run, got %q", commands.calls)
	}
}

func TestDrudgerService_RunTask_GivesEachRepositoryItsOwnHooksDirectory(t *testing.T) {
	cases := []struct {
		name  string
		paths []string
		// wantHooks is the hooks directory of each repository, in config order.
		wantHooks []string
	}{
		{
			name:      "a project that is itself a repository",
			paths:     []string{"."},
			wantHooks: []string{".drudge/hooks/%2E"},
		},
		{
			name:      "a nested path stays apart from a flat name that looks like it",
			paths:     []string{"services/api", "services%2Fapi", "services-api"},
			wantHooks: []string{".drudge/hooks/services%2Fapi", ".drudge/hooks/services%252Fapi", ".drudge/hooks/services-api"},
		},
		{
			name:      "a leading dot is escaped and an inner one is kept",
			paths:     []string{".config", "web.app"},
			wantHooks: []string{".drudge/hooks/%2Econfig", ".drudge/hooks/web.app"},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			projectDir := setupProjectDir(t)
			taskToRun := todoTask()
			settings := testSettings()
			for _, path := range testCase.paths {
				settings.Repositories = append(settings.Repositories, project.Repository{Path: path, Setup: []string{"make hooks"}})
			}
			commands := &fakeCommandRunner{projectDir: projectDir, outputs: []string{sandboxListingWith()}}
			service := newTestServiceWith(settings, commands, taskToRun)

			err := service.RunTask(testProjectSlug, taskToRun.ID, false)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			wantCalls := [][]string{resetCall(testCase.wantHooks...)}
			for index, path := range testCase.paths {
				wantCalls = append(wantCalls, setupCall(projectDir, path, testCase.wantHooks[index], "make hooks"))
			}
			calls := commands.calls
			if got := calls[2 : len(calls)-1]; !slices.EqualFunc(got, wantCalls, slices.Equal[[]string]) {
				t.Errorf("expected the reset and setup calls %q, got %q", wantCalls, got)
			}
		})
	}
}

// Environment variables the tests pass into the sandbox.
const (
	testTokenEnv    = "DRUDGE_TEST_AUTH_TOKEN"
	testRegistryEnv = "DRUDGE_TEST_REGISTRY"
	testTokenValue  = "s3cret-token"
)

// settingsWithEnv are the settings of a project of the setup repositories that
// passes the given variables into the sandbox.
func settingsWithEnv(names ...string) Settings {
	settings := settingsWithSetup()
	settings.EnvNames = names
	return settings
}

// unsetEnv unsets a variable for the length of a test.
func unsetEnv(t *testing.T, name string) {
	t.Helper()
	t.Setenv(name, "")
	if err := os.Unsetenv(name); err != nil {
		t.Fatalf("could not unset %s: %v", name, err)
	}
}

// envFlags are the sbx exec flags that pass the given variables.
func envFlags(names ...string) []string {
	var flags []string
	for _, name := range names {
		flags = append(flags, sbxEnvFlag, name)
	}
	return flags
}

func TestDrudgerService_RunTask_PassesTheEnvToTheSandbox(t *testing.T) {
	cases := []struct {
		name     string
		envNames []string
	}{
		{name: "no variables pass no flags"},
		{name: "one variable", envNames: []string{testTokenEnv}},
		{name: "several variables in config order", envNames: []string{testTokenEnv, testRegistryEnv}},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			for _, name := range testCase.envNames {
				t.Setenv(name, testTokenValue)
			}
			projectDir := setupProjectDir(t)
			taskToRun := todoTask()
			taskToRun.Repositories = []string{"ui"}
			commands := &fakeCommandRunner{projectDir: projectDir, outputs: []string{sandboxListingWith()}}
			service := newTestServiceWith(settingsWithEnv(testCase.envNames...), commands, taskToRun)

			if err := service.RunTask(testProjectSlug, taskToRun.ID, false); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			flags := envFlags(testCase.envNames...)
			withoutEnv := setupCall(projectDir, "ui", ".drudge/hooks/ui", "pnpm install")
			wantSetup := slices.Concat(withoutEnv[:4], flags, withoutEnv[4:])
			if got := commands.calls[3]; !slices.Equal(got, wantSetup) {
				t.Errorf("expected the setup call %q, got %q", wantSetup, got)
			}
			wantStart := slices.Concat([]string{sbxBinary, sbxExecSubcommand, sbxDetachedFlag}, flags, []string{testSandbox})
			if got := commands.started[0]; !slices.Equal(got[:len(wantStart)], wantStart) {
				t.Errorf("expected the agent start to begin with %q, got %q", wantStart, got)
			}
			for _, call := range commands.calls {
				if slices.ContainsFunc(call, func(arg string) bool { return strings.Contains(arg, testTokenValue) }) {
					t.Errorf("expected no command line to hold the value of a variable, got %q", call)
				}
			}
			if got := fmt.Sprintf("%+v", service.progress.events); strings.Contains(got, testTokenValue) {
				t.Errorf("expected no progress event to hold the value of a variable, got %s", got)
			}
		})
	}
}

func TestDrudgerService_RunTask_UnsetEnvStopsTheLaunchBeforeSetup(t *testing.T) {
	cases := []struct {
		name       string
		setNames   []string
		unsetNames []string
		wantInErr  []string
	}{
		{
			name:       "one variable not set",
			setNames:   []string{testTokenEnv},
			unsetNames: []string{testRegistryEnv},
			wantInErr:  []string{testRegistryEnv, EnvKey},
		},
		{
			name:       "every variable not set",
			unsetNames: []string{testTokenEnv, testRegistryEnv},
			wantInErr:  []string{testTokenEnv, testRegistryEnv, EnvKey},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			for _, name := range testCase.setNames {
				t.Setenv(name, testTokenValue)
			}
			for _, name := range testCase.unsetNames {
				unsetEnv(t, name)
			}
			projectDir := setupProjectDir(t)
			taskToRun := todoTask()
			commands := &fakeCommandRunner{projectDir: projectDir, outputs: []string{sandboxListingWith()}}
			service := newTestServiceWith(settingsWithEnv(testTokenEnv, testRegistryEnv), commands, taskToRun)

			err := service.RunTask(testProjectSlug, taskToRun.ID, false)

			if err == nil {
				t.Fatal("expected an unset variable to stop the launch")
			}
			for _, want := range testCase.wantInErr {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("expected the error to name %q, got %q", want, err)
				}
			}
			if strings.Contains(err.Error(), testTokenValue) {
				t.Errorf("expected the error to leave out the value of a variable, got %q", err)
			}
			if commands.calls != nil || commands.started != nil {
				t.Errorf("expected nothing to run, got calls %q and starts %q", commands.calls, commands.started)
			}
			if index := firstEventIndex[SetupCommandStarted](service.progress); index >= 0 {
				t.Errorf("expected no setup to start, got %+v", service.progress.events)
			}
			if taskToRun.Status != task.StatusTodo {
				t.Errorf("expected the task to stay %q, got %q", task.StatusTodo, taskToRun.Status)
			}
			if held := service.drudgers.holderOf(taskToRun.ID); held != nil {
				t.Errorf("expected no Drudger to hold the task, got Drudger %d", held.Slot)
			}
		})
	}
}

func TestDrudgerService_RunTask_DryRunReportsTheEnv(t *testing.T) {
	cases := []struct {
		name      string
		envNames  []string
		setNames  []string
		wantUnset []string
	}{
		{name: "no variables"},
		{
			name:     "every variable set",
			envNames: []string{testTokenEnv, testRegistryEnv},
			setNames: []string{testTokenEnv, testRegistryEnv},
		},
		{
			name:      "a variable not set",
			envNames:  []string{testTokenEnv, testRegistryEnv},
			setNames:  []string{testTokenEnv},
			wantUnset: []string{testRegistryEnv},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			unsetEnv(t, testTokenEnv)
			unsetEnv(t, testRegistryEnv)
			for _, name := range testCase.setNames {
				t.Setenv(name, testTokenValue)
			}
			projectDir := setupProjectDir(t)
			taskToRun := todoTask()
			commands := &fakeCommandRunner{projectDir: projectDir}
			service := newTestServiceWith(settingsWithEnv(testCase.envNames...), commands, taskToRun)

			if err := service.RunTask(testProjectSlug, taskToRun.ID, true); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			described := singleEvent[RunDescribed](t, service.progress)
			if !slices.Equal(described.EnvNames, testCase.envNames) {
				t.Errorf("expected the variables %q, got %q", testCase.envNames, described.EnvNames)
			}
			if !slices.Equal(described.UnsetEnvNames, testCase.wantUnset) {
				t.Errorf("expected the unset variables %q, got %q", testCase.wantUnset, described.UnsetEnvNames)
			}
			if commands.calls != nil {
				t.Errorf("expected nothing to run, got %q", commands.calls)
			}
		})
	}
}
