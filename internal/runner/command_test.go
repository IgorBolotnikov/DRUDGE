package runner

import (
	"slices"
	"strings"
	"testing"

	"drudge/internal/config"
)

func TestPickRunnerCommand(t *testing.T) {
	const prompt = "implement it\nplease"

	cases := []struct {
		name            string
		env             config.Env
		harness         config.Harness
		runnerID        int
		want            []string
		wantErrContains string
	}{
		{
			name:     "sbx with claude code",
			env:      config.EnvDockerSbx,
			harness:  config.HarnessClaudeCode,
			runnerID: 1,
			want:     []string{"sbx", "run", "claude", "--name", "drudge-claude-test-project-1", "--detached", "--", "-p", prompt},
		},
		{
			name:     "sbx with opencode",
			env:      config.EnvDockerSbx,
			harness:  config.HarnessOpencode,
			runnerID: 2,
			want:     []string{"sbx", "run", "opencode", "--name", "drudge-opencode-test-project-2", "--detached", "--", "-p", prompt},
		},
		{
			name:            "unknown harness is an error",
			env:             config.EnvDockerSbx,
			harness:         config.Harness("codex"),
			runnerID:        1,
			wantErrContains: "codex",
		},
		{
			name:            "unknown environment is an error",
			env:             config.Env("bare-metal"),
			harness:         config.HarnessClaudeCode,
			runnerID:        1,
			wantErrContains: "bare-metal",
		},
		{
			name:            "empty runner settings are an error",
			runnerID:        1,
			wantErrContains: "runner settings",
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			service := newTestServiceWithConfigs(
				&config.LocalConfig{ProjectSlug: testProjectSlug},
				&config.GlobalConfig{Runner: config.RunnerConfig{Env: testCase.env, Harness: testCase.harness}},
			)

			argv, err := service.pickRunnerCommand(testProjectSlug, testCase.runnerID, prompt)

			if testCase.wantErrContains != "" {
				if err == nil {
					t.Fatalf("expected an error naming %s, got argv %v", testCase.wantErrContains, argv)
				}
				if !strings.Contains(err.Error(), testCase.wantErrContains) {
					t.Errorf("expected error to name %s, got %q", testCase.wantErrContains, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !slices.Equal(argv, testCase.want) {
				t.Errorf("expected argv %v, got %v", testCase.want, argv)
			}
		})
	}
}

func TestFormatRunnerName(t *testing.T) {
	cases := []struct {
		name        string
		projectSlug string
		harness     config.Harness
		runnerID    int
		want        string
	}{
		{name: "claude code", projectSlug: "drudge", harness: config.HarnessClaudeCode, runnerID: 1, want: "drudge-claude-drudge-1"},
		{name: "opencode", projectSlug: "drudge", harness: config.HarnessOpencode, runnerID: 12, want: "drudge-opencode-drudge-12"},
		{name: "unknown harness", projectSlug: "drudge", harness: config.Harness("codex"), runnerID: 3, want: "drudge-unknown-drudge-3"},
		{
			name:        "two projects get two names for the same slot",
			projectSlug: "other-project",
			harness:     config.HarnessClaudeCode,
			runnerID:    1,
			want:        "drudge-claude-other-project-1",
		},
		{
			name:        "a slug sbx would reject is normalized",
			projectSlug: "My_Project!",
			harness:     config.HarnessClaudeCode,
			runnerID:    1,
			want:        "drudge-claude-my-project-1",
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			got := formatRunnerName(testCase.projectSlug, testCase.runnerID, testCase.harness)
			if got != testCase.want {
				t.Errorf("expected runner name %q, got %q", testCase.want, got)
			}
		})
	}
}

func TestSandboxNameSlug(t *testing.T) {
	cases := []struct {
		name        string
		projectSlug string
		want        string
	}{
		{name: "a plain slug is left alone", projectSlug: "drudge", want: "drudge"},
		{name: "hyphens and periods survive", projectSlug: "drudge-api.v2", want: "drudge-api.v2"},
		{name: "upper case is folded down", projectSlug: "My Project", want: "my-project"},
		{name: "underscores become hyphens", projectSlug: "my_project", want: "my-project"},
		{name: "runs of rejected characters collapse", projectSlug: "my // project", want: "my-project"},
		{name: "separators are trimmed off the ends", projectSlug: "  .drudge- ", want: "drudge"},
		{name: "non-ascii is folded into separators", projectSlug: "проект-drudge", want: "drudge"},
		{name: "a slug with nothing usable falls back", projectSlug: "!!!", want: unknownProjectSlug},
		{name: "an empty slug falls back", projectSlug: "", want: unknownProjectSlug},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			got := sandboxNameSlug(testCase.projectSlug)
			if got != testCase.want {
				t.Errorf("expected slug %q, got %q", testCase.want, got)
			}
		})
	}
}

func TestFormatArgv(t *testing.T) {
	cases := []struct {
		name string
		argv []string
		want string
	}{
		{name: "empty argv", argv: nil, want: ""},
		{name: "quotes every element", argv: []string{"sbx", "run"}, want: `"sbx" "run"`},
		{
			name: "keeps a multi line argument on one line",
			argv: []string{"sbx", "-p", "first\nsecond"},
			want: `"sbx" "-p" "first\nsecond"`,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			got := formatArgv(testCase.argv)
			if got != testCase.want {
				t.Errorf("expected %s, got %s", testCase.want, got)
			}
		})
	}
}
