package drudger

import (
	"slices"
	"strings"
	"testing"

	"github.com/IgorBolotnikov/DRUDGE/internal/common"
)

// runTaskFor drives a run to completion in a project directory against a given
// sandbox listing, and returns the commands it issued.
func runTaskFor(t *testing.T, settings Settings, projectDir, listing string) *fakeCommandRunner {
	t.Helper()
	taskToRun := todoTask()
	taskToRun.ProjectSlug = settings.ProjectSlug
	commands := &fakeCommandRunner{projectDir: projectDir, outputs: []string{listing}}
	service := newTestServiceWith(settings, commands, taskToRun)

	err := service.RunTask(settings.ProjectSlug, taskToRun.ID, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	return commands
}

func TestDrudgerService_RunTask_IssuesTheSbxCommands(t *testing.T) {
	projectDir := setupProjectDir(t)
	commands := runTaskFor(t, testSettings(), projectDir, sandboxListingWith())

	wantInspect := []string{"sbx", "ls", "--json"}
	if got := commands.call(sbxLsSubcommand); !slices.Equal(got, wantInspect) {
		t.Errorf("expected inspect %v, got %v", wantInspect, got)
	}

	wantCreate := append([]string{"sbx", "create", "claude"}, slotMounts(projectDir, 1)...)
	wantCreate = append(wantCreate, "--name", testSandbox)
	if got := commands.call(sbxCreateSubcommand); !slices.Equal(got, wantCreate) {
		t.Errorf("expected create %v, got %v", wantCreate, got)
	}

	start := commands.call(sbxExecSubcommand)
	wantStartPrefix := []string{"sbx", "exec", "-d", testSandbox, "sh", "-c"}
	if len(start) != len(wantStartPrefix)+1 {
		t.Fatalf("expected the launcher as the last argument of start, got %v", start)
	}
	if got := start[:len(wantStartPrefix)]; !slices.Equal(got, wantStartPrefix) {
		t.Errorf("expected start to begin with %v, got %v", wantStartPrefix, got)
	}
}

func TestDrudgerService_RunTask_UnsupportedDrudgerSettings(t *testing.T) {
	cases := []struct {
		name            string
		env             Env
		harness         Harness
		wantErrContains string
	}{
		{name: "opencode is not wired up yet", env: EnvDockerSbx, harness: HarnessOpencode, wantErrContains: "opencode"},
		{name: "unknown harness", env: EnvDockerSbx, harness: Harness("codex"), wantErrContains: "codex"},
		{name: "unknown environment", env: Env("bare-metal"), harness: HarnessClaudeCode, wantErrContains: "bare-metal"},
		{name: "empty Drudger settings", wantErrContains: "Drudger settings"},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			setupProjectDir(t)
			commands := &fakeCommandRunner{}
			settings := testSettings()
			settings.Env = testCase.env
			settings.Harness = testCase.harness
			service := newTestServiceWith(
				settings,
				commands,
				todoTask(),
			)

			err := service.RunTask(testProjectSlug, "task-1", false)
			if err == nil {
				t.Fatalf("expected an error naming %s", testCase.wantErrContains)
			}
			if !strings.Contains(err.Error(), testCase.wantErrContains) {
				t.Errorf("expected error to name %s, got %q", testCase.wantErrContains, err)
			}
			if commands.calls != nil {
				t.Errorf("expected nothing to be run, got %v", commands.subcommands())
			}
		})
	}
}

func TestDrudgerService_RunTask_LauncherRunsTheAgentOverTheRunDirectory(t *testing.T) {
	projectDir := setupProjectDir(t)
	commands := runTaskFor(t, testSettings(), projectDir, sandboxListingWith(testSandbox))

	start := commands.call(sbxExecSubcommand)
	launcher := start[len(start)-1]
	runDir := common.RunDir(projectDir, "task-1")

	want := []string{
		"cd '" + slotRoot(projectDir, 1) + "' || exit 1",
		`claude -p "$(cat '` + runDir + `/prompt.txt')"`,
		"--output-format stream-json",
		"--verbose",
		"--permission-mode bypassPermissions",
		"> '" + runDir + "/stream.jsonl'",
		"2> '" + runDir + "/stderr.log'",
		"echo $? > '" + runDir + "/exit'",
	}
	for _, fragment := range want {
		if !strings.Contains(launcher, fragment) {
			t.Errorf("expected the launcher to contain %q, got %q", fragment, launcher)
		}
	}
}

func TestDrudgerService_RunTask_LauncherQuotesAwkwardWorkspacePaths(t *testing.T) {
	cases := []struct {
		name    string
		dirName string
	}{
		{name: "a space in the path", dirName: "my repo"},
		{name: "a single quote in the path", dirName: "igor's repo"},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			projectDir := setupProjectDirNamed(t, testCase.dirName)
			commands := runTaskFor(t, testSettings(), projectDir, sandboxListingWith(testSandbox))

			start := commands.call(sbxExecSubcommand)
			launcher := start[len(start)-1]

			// A shell must read the path back whole. Single quotes do that for
			// everything but a single quote, which has to be broken out.
			quoted := "'" + strings.ReplaceAll(slotRoot(projectDir, 1), "'", `'\''`) + "'"
			if !strings.Contains(launcher, "cd "+quoted+" || exit 1") {
				t.Errorf("expected the launcher to cd to %s, got %q", quoted, launcher)
			}
		})
	}
}

func TestDrudgerService_RunTask_NamesASandboxPerProject(t *testing.T) {
	cases := []struct {
		name        string
		projectSlug string
		wantSandbox string
	}{
		{name: "a plain slug is left alone", projectSlug: "drudge", wantSandbox: "drudge-claude-drudge-1"},
		{name: "two projects get two names for the same slot", projectSlug: "other-project", wantSandbox: "drudge-claude-other-project-1"},
		{name: "hyphens and periods survive", projectSlug: "drudge-api.v2", wantSandbox: "drudge-claude-drudge-api.v2-1"},
		{name: "upper case is folded down", projectSlug: "My Project", wantSandbox: "drudge-claude-my-project-1"},
		{name: "underscores sbx rejects become hyphens", projectSlug: "my_project", wantSandbox: "drudge-claude-my-project-1"},
		{name: "runs of rejected characters collapse", projectSlug: "my // project", wantSandbox: "drudge-claude-my-project-1"},
		{name: "separators are trimmed off the ends", projectSlug: "  .drudge- ", wantSandbox: "drudge-claude-drudge-1"},
		{name: "non-ascii is folded into separators", projectSlug: "проект-drudge", wantSandbox: "drudge-claude-drudge-1"},
		{name: "a slug with nothing usable falls back", projectSlug: "!!!", wantSandbox: "drudge-claude-unknown-1"},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			projectDir := setupProjectDir(t)
			settings := testSettings()
			settings.ProjectSlug = testCase.projectSlug
			commands := runTaskFor(t, settings, projectDir, sandboxListingWith(testCase.wantSandbox))

			start := commands.call(sbxExecSubcommand)
			if !slices.Contains(start, testCase.wantSandbox) {
				t.Errorf("expected the start call to name sandbox %q, got %v", testCase.wantSandbox, start)
			}
			if commands.call(sbxCreateSubcommand) != nil {
				t.Errorf("expected the listed sandbox to be reused, got %v", commands.subcommands())
			}
		})
	}
}

func TestDrudgerService_RunTask_DryRunPreviewsEverythingAndWritesNothing(t *testing.T) {
	projectDir := setupProjectDir(t)
	taskToRun := todoTask()
	taskToRun.TicketID = "PROJ-123"
	commands := &fakeCommandRunner{}
	service := newTestServiceWith(testSettings(), commands, taskToRun)

	if err := service.RunTask(testProjectSlug, taskToRun.ID, true); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	described := singleEvent[RunDescribed](t, service.progress)
	if described.Task.ID != taskToRun.ID || described.Slot != 1 || described.Sandbox != testSandbox {
		t.Errorf("expected task %s to preview on Drudger 1 (%s), got %+v", taskToRun.ID, testSandbox, described)
	}
	for _, want := range []string{taskToRun.Title, taskToRun.Description, taskToRun.TicketID} {
		if !strings.Contains(described.Prompt, want) {
			t.Errorf("expected the prompt to contain %q, got %q", want, described.Prompt)
		}
	}

	wantSubcommands := []string{sbxLsSubcommand, sbxCreateSubcommand, sbxExecSubcommand}
	if len(described.Commands) != len(wantSubcommands) {
		t.Fatalf("expected the commands %v, got %v", wantSubcommands, described.Commands)
	}
	for index, argv := range described.Commands {
		if argv[0] != sbxBinary || !slices.Contains(argv, wantSubcommands[index]) {
			t.Errorf("expected command %d to be sbx %s, got %v", index+1, wantSubcommands[index], argv)
		}
	}
	if create := described.Commands[1]; !slices.Contains(create, slotRoot(projectDir, 1)) {
		t.Errorf("expected the create command to mount %s, got %v", slotRoot(projectDir, 1), create)
	}
	if start := described.Commands[2]; !slices.Contains(start, sbxDetachedFlag) {
		t.Errorf("expected the start command to hold the argument %q, got %v", sbxDetachedFlag, start)
	}

	if commands.calls != nil {
		t.Errorf("expected a dry run not to run anything, got %v", commands.subcommands())
	}
	if len(service.runs.runs) != 0 {
		t.Errorf("expected a dry run not to write a run directory, got %v", service.runs.runs)
	}
}
