package drudger

import (
	"os"
	"os/exec"
	"path/filepath"
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

	start := commands.started[0]
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

func TestDrudgerService_RunTask_HarnessWithoutAStreamReader(t *testing.T) {
	setupProjectDir(t)
	commands := &fakeCommandRunner{}
	service := newTestServiceWith(testSettings(), commands, todoTask())
	service.streams = map[Harness]StreamReader{}

	err := service.RunTask(testProjectSlug, "task-1", false)
	if err == nil {
		t.Fatal("expected an error for a harness with no stream reader")
	}
	if !strings.Contains(err.Error(), string(HarnessClaudeCode)) {
		t.Errorf("expected the error to name harness %s, got %q", HarnessClaudeCode, err)
	}
	if commands.calls != nil {
		t.Errorf("expected nothing to be run, got %v", commands.subcommands())
	}
	if len(service.drudgers.drudgers) != 0 {
		t.Errorf("expected no Drudger to be claimed, got %+v", service.drudgers.drudgers)
	}
}

func TestDrudgerService_RunTask_LauncherRunsTheAgentOverTheRunDirectory(t *testing.T) {
	projectDir := setupProjectDir(t)
	commands := runTaskFor(t, testSettings(), projectDir, sandboxListingWith(testSandbox))

	start := commands.started[0]
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

func TestDrudgerService_RunTask_LauncherPutsTheListedDirectoriesOnPath(t *testing.T) {
	cases := []struct {
		name string
		// pathFile is what the path file holds. Nil leaves the file out.
		pathFile *string
		// wantPrefix is what the launcher puts before the PATH it was given.
		wantPrefix string
	}{
		{name: "a missing path file"},
		{name: "an empty path file", pathFile: new("")},
		{
			name:       "directories in file order",
			pathFile:   new("/nonexistent/mise/shims\n/nonexistent/.local/bin\n"),
			wantPrefix: "/nonexistent/mise/shims:/nonexistent/.local/bin:",
		},
		{
			name:       "empty lines and a missing last newline",
			pathFile:   new("\n/nonexistent/first\n\n/nonexistent/last"),
			wantPrefix: "/nonexistent/first:/nonexistent/last:",
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			projectDir := setupProjectDir(t)
			commands := runTaskFor(t, testSettings(), projectDir, sandboxListingWith(testSandbox))
			launcher := commands.started[0][len(commands.started[0])-1]

			sandboxHome := t.TempDir()
			if testCase.pathFile != nil {
				writeTestFile(t, filepath.Join(sandboxHome, sandboxPathFile), *testCase.pathFile)
			}
			// The agent stands in for claude and writes its PATH to the stream.
			agentDir := t.TempDir()
			agentPath := filepath.Join(agentDir, claudeBinary)
			writeTestFile(t, agentPath, "#!/bin/sh\nprintf %s \"$PATH\"\n")
			if err := os.Chmod(agentPath, 0o755); err != nil {
				t.Fatalf("could not make the agent executable: %v", err)
			}
			givenPath := agentDir + ":/usr/bin:/bin"
			runDir := common.RunDir(projectDir, "task-1")
			for _, dir := range []string{runDir, slotRoot(projectDir, 1)} {
				if err := common.EnsureDir(dir); err != nil {
					t.Fatalf("could not create %s: %v", dir, err)
				}
			}

			shell := exec.Command(shellBinary, shellCommandFlag, launcher)
			shell.Env = []string{"HOME=" + sandboxHome, "PATH=" + givenPath}
			if output, err := shell.CombinedOutput(); err != nil {
				t.Fatalf("the launcher failed: %v: %s", err, output)
			}

			got, err := os.ReadFile(common.RunStreamPath(runDir))
			if err != nil {
				t.Fatalf("could not read the stream: %v", err)
			}
			if want := testCase.wantPrefix + givenPath; string(got) != want {
				t.Errorf("expected the agent to get PATH %q, got %q", want, got)
			}
		})
	}
}

func TestDrudgerService_RunTask_LauncherPassesTheModel(t *testing.T) {
	cases := []struct {
		name        string
		model       string
		wantCommand string
	}{
		{
			name:        "no model leaves the choice to the harness",
			wantCommand: "--permission-mode bypassPermissions > ",
		},
		{
			name:        "an alias",
			model:       "opus",
			wantCommand: "--permission-mode bypassPermissions --model 'opus' > ",
		},
		{
			name:        "a full model name",
			model:       "claude-opus-5-5",
			wantCommand: "--permission-mode bypassPermissions --model 'claude-opus-5-5' > ",
		},
		{
			name:        "a model with a single quote",
			model:       "it's",
			wantCommand: `--permission-mode bypassPermissions --model 'it'\''s' > `,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			projectDir := setupProjectDir(t)
			settings := testSettings()
			settings.Model = testCase.model
			commands := runTaskFor(t, settings, projectDir, sandboxListingWith(testSandbox))

			start := commands.started[0]
			launcher := start[len(start)-1]
			if !strings.Contains(launcher, testCase.wantCommand) {
				t.Errorf("expected the launcher to contain %q, got %q", testCase.wantCommand, launcher)
			}
		})
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

			start := commands.started[0]
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

			start := commands.started[0]
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

	wantSubcommands := []string{sbxLsSubcommand, sbxCreateSubcommand, sbxExecSubcommand, sbxExecSubcommand}
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
	if start := described.Commands[3]; !slices.Contains(start, sbxDetachedFlag) {
		t.Errorf("expected the start command to hold the argument %q, got %v", sbxDetachedFlag, start)
	}

	if commands.calls != nil {
		t.Errorf("expected a dry run not to run anything, got %v", commands.subcommands())
	}
	if len(service.runs.runs) != 0 {
		t.Errorf("expected a dry run not to write a run directory, got %v", service.runs.runs)
	}
}
