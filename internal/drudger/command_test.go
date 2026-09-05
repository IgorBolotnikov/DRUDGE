package drudger

import (
	"slices"
	"strconv"
	"strings"
	"testing"

	"drudge/internal/common"
	"drudge/internal/config"
)

// runTaskFor drives a run to completion in a workspace against a given
// sandbox listing, and returns the commands it issued.
func runTaskFor(t *testing.T, localCfg *config.LocalConfig, workspace, listing string) *fakeCommandRunner {
	t.Helper()
	taskToRun := todoTask()
	taskToRun.ProjectSlug = localCfg.ProjectSlug
	commands := &fakeCommandRunner{workspace: workspace, outputs: []string{listing}}
	service := newTestServiceWith(localCfg, config.DefaultConfig(), commands, taskToRun)

	var err error
	captureOutput(func() { err = service.RunTask(localCfg.ProjectSlug, taskToRun.ID, false) })
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	return commands
}

func TestDrudgerService_RunTask_IssuesTheSbxCommands(t *testing.T) {
	workspace := setupWorkspace(t)
	commands := runTaskFor(t, &config.LocalConfig{ProjectSlug: testProjectSlug}, workspace, sandboxListingWith())

	wantInspect := []string{"sbx", "ls", "--json"}
	if got := commands.call(sbxLsSubcommand); !slices.Equal(got, wantInspect) {
		t.Errorf("expected inspect %v, got %v", wantInspect, got)
	}

	wantCreate := []string{"sbx", "create", "claude", workspace, "--name", testSandbox}
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
		env             config.Env
		harness         config.Harness
		wantErrContains string
	}{
		{name: "opencode is not wired up yet", env: config.EnvDockerSbx, harness: config.HarnessOpencode, wantErrContains: "opencode"},
		{name: "unknown harness", env: config.EnvDockerSbx, harness: config.Harness("codex"), wantErrContains: "codex"},
		{name: "unknown environment", env: config.Env("bare-metal"), harness: config.HarnessClaudeCode, wantErrContains: "bare-metal"},
		{name: "empty Drudger settings", wantErrContains: "Drudger settings"},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			setupWorkspace(t)
			commands := &fakeCommandRunner{}
			service := newTestServiceWith(
				&config.LocalConfig{ProjectSlug: testProjectSlug},
				&config.GlobalConfig{Drudger: config.DrudgerConfig{Env: testCase.env, Harness: testCase.harness}},
				commands,
				todoTask(),
			)

			var err error
			captureOutput(func() { err = service.RunTask(testProjectSlug, "task-1", false) })
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
	workspace := setupWorkspace(t)
	commands := runTaskFor(t, &config.LocalConfig{ProjectSlug: testProjectSlug}, workspace, sandboxListingWith(testSandbox))

	start := commands.call(sbxExecSubcommand)
	launcher := start[len(start)-1]
	runDir := common.RunDir(workspace, "task-1")

	want := []string{
		"cd '" + workspace + "' || exit 1",
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
			workspace := setupWorkspaceNamed(t, testCase.dirName)
			commands := runTaskFor(t, &config.LocalConfig{ProjectSlug: testProjectSlug}, workspace, sandboxListingWith(testSandbox))

			start := commands.call(sbxExecSubcommand)
			launcher := start[len(start)-1]

			// A shell must read the path back whole. Single quotes do that for
			// everything but a single quote, which has to be broken out.
			quoted := "'" + strings.ReplaceAll(workspace, "'", `'\''`) + "'"
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
			workspace := setupWorkspace(t)
			commands := runTaskFor(t, &config.LocalConfig{ProjectSlug: testCase.projectSlug}, workspace, sandboxListingWith(testCase.wantSandbox))

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
	workspace := setupWorkspace(t)
	taskToRun := todoTask()
	taskToRun.TicketID = "PROJ-123"
	commands := &fakeCommandRunner{}
	service := newTestServiceWith(&config.LocalConfig{ProjectSlug: testProjectSlug}, config.DefaultConfig(), commands, taskToRun)

	var err error
	out := captureOutput(func() { err = service.RunTask(testProjectSlug, taskToRun.ID, true) })
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, want := range []string{taskToRun.Title, taskToRun.Description, taskToRun.TicketID, testSandbox} {
		if !strings.Contains(out, want) {
			t.Errorf("expected the preview to contain %q, got %q", want, out)
		}
	}

	// Every argument is printed quoted, so a launcher full of newlines stays
	// on one line of the preview.
	for _, argument := range []string{sbxBinary, sbxLsSubcommand, sbxCreateSubcommand, sbxExecSubcommand, sbxDetachedFlag, workspace} {
		if !strings.Contains(out, strconv.Quote(argument)) {
			t.Errorf("expected the preview to contain the argument %q, got %q", argument, out)
		}
	}

	if commands.calls != nil {
		t.Errorf("expected a dry run not to run anything, got %v", commands.subcommands())
	}
	if exists, err := common.Exists(common.LocalRunsDir()); err != nil || exists {
		t.Errorf("expected a dry run not to write a run directory, exists %t (%v)", exists, err)
	}
}
