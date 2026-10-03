package cmd

import (
	"reflect"
	"testing"
	"time"

	"github.com/IgorBolotnikov/DRUDGE/internal/config"
	"github.com/IgorBolotnikov/DRUDGE/internal/drudger"
	"github.com/IgorBolotnikov/DRUDGE/internal/project"
)

func TestNewDrudgerSettings(t *testing.T) {
	const projectSlug = "drudge"
	repositories := []project.Repository{{Path: "api"}, {Path: "ui", DefaultBranch: "trunk"}}

	customGlobal := config.DefaultConfig()
	customGlobal.Drudger.Env = config.Env("bare-metal")
	customGlobal.Drudger.Harness = config.HarnessOpencode
	customGlobal.Drudger.MaxConcurrentDrudgers = 5
	customGlobal.Drudger.SandboxTimeouts = config.SandboxTimeouts{ListSeconds: 5, CreateSeconds: 60, RemoveSeconds: 7}

	cases := []struct {
		name         string
		local        *config.LocalConfig
		global       *config.GlobalConfig
		pullRequests config.PullRequestSettings
		want         drudger.Settings
	}{
		{
			name:   "the default configs",
			local:  &config.LocalConfig{ProjectSlug: projectSlug, Repositories: repositories},
			global: config.DefaultConfig(),
			want: drudger.Settings{
				ProjectSlug:           projectSlug,
				Repositories:          repositories,
				Env:                   drudger.EnvDockerSbx,
				Harness:               drudger.HarnessClaudeCode,
				MaxConcurrentDrudgers: 3,
				SandboxTimeouts: drudger.SandboxTimeouts{
					List:   30 * time.Second,
					Create: 10 * time.Minute,
					Remove: 2 * time.Minute,
				},
			},
		},
		{
			name:   "a configured environment, harness, limit and timeouts",
			local:  &config.LocalConfig{ProjectSlug: projectSlug, Repositories: repositories},
			global: customGlobal,
			want: drudger.Settings{
				ProjectSlug:           projectSlug,
				Repositories:          repositories,
				Env:                   drudger.Env("bare-metal"),
				Harness:               drudger.HarnessOpencode,
				MaxConcurrentDrudgers: 5,
				SandboxTimeouts: drudger.SandboxTimeouts{
					List:   5 * time.Second,
					Create: time.Minute,
					Remove: 7 * time.Second,
				},
			},
		},
		{
			name:   "the local limit wins over the global one",
			local:  &config.LocalConfig{ProjectSlug: projectSlug, Repositories: repositories, MaxConcurrentDrudgers: 2},
			global: customGlobal,
			want: drudger.Settings{
				ProjectSlug:           projectSlug,
				Repositories:          repositories,
				Env:                   drudger.Env("bare-metal"),
				Harness:               drudger.HarnessOpencode,
				MaxConcurrentDrudgers: 2,
				SandboxTimeouts: drudger.SandboxTimeouts{
					List:   5 * time.Second,
					Create: time.Minute,
					Remove: 7 * time.Second,
				},
			},
		},
		{
			name:   "the pull request settings",
			local:  &config.LocalConfig{ProjectSlug: projectSlug, Repositories: repositories},
			global: config.DefaultConfig(),
			pullRequests: config.PullRequestSettings{
				IsEnabled:    true,
				IsDraft:      true,
				TitleFormat:  "{{ticketID}}: <summary>",
				TemplatePath: "template.md",
				StepsPath:    "steps.md",
			},
			want: drudger.Settings{
				ProjectSlug:           projectSlug,
				Repositories:          repositories,
				Env:                   drudger.EnvDockerSbx,
				Harness:               drudger.HarnessClaudeCode,
				MaxConcurrentDrudgers: 3,
				SandboxTimeouts: drudger.SandboxTimeouts{
					List:   30 * time.Second,
					Create: 10 * time.Minute,
					Remove: 2 * time.Minute,
				},
				PullRequests: drudger.PullRequestSettings{
					IsDraft:      true,
					TitleFormat:  "{{ticketID}}: <summary>",
					TemplatePath: "template.md",
					StepsPath:    "steps.md",
				},
			},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			got, err := newDrudgerSettings(testCase.local, testCase.global, testCase.pullRequests)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(got, testCase.want) {
				t.Errorf("expected settings %+v, got %+v", testCase.want, got)
			}
		})
	}
}
