package cmd

import (
	"reflect"
	"testing"
	"time"

	"github.com/IgorBolotnikov/DRUDGE/internal/adapters/persistence"
	"github.com/IgorBolotnikov/DRUDGE/internal/config"
	"github.com/IgorBolotnikov/DRUDGE/internal/drudger"
	"github.com/IgorBolotnikov/DRUDGE/internal/project"
	"github.com/IgorBolotnikov/DRUDGE/internal/task"
)

func TestNewDrudgerSettings(t *testing.T) {
	const projectSlug = "drudge"
	repositories := []project.Repository{{Path: "api", Setup: []string{"npm ci"}}, {Path: "ui", DefaultBranch: "trunk"}}

	customGlobal := config.DefaultConfig()
	customGlobal.Drudger.Env = config.Env("bare-metal")
	customGlobal.Drudger.Harness = config.HarnessOpencode
	customGlobal.Drudger.MaxConcurrentDrudgers = 5
	customGlobal.Drudger.SandboxTimeouts = config.SandboxTimeouts{ListSeconds: 5, CreateSeconds: 60, RemoveSeconds: 7, SetupSeconds: 300, FirstOutputSeconds: 180}

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
				BranchFormat:          task.DefaultBranchFormat,
				SandboxTimeouts: drudger.SandboxTimeouts{
					List:        30 * time.Second,
					Create:      10 * time.Minute,
					Remove:      2 * time.Minute,
					Setup:       15 * time.Minute,
					FirstOutput: time.Minute,
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
				BranchFormat:          task.DefaultBranchFormat,
				SandboxTimeouts: drudger.SandboxTimeouts{
					List:        5 * time.Second,
					Create:      time.Minute,
					Remove:      7 * time.Second,
					Setup:       5 * time.Minute,
					FirstOutput: 3 * time.Minute,
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
				BranchFormat:          task.DefaultBranchFormat,
				SandboxTimeouts: drudger.SandboxTimeouts{
					List:        5 * time.Second,
					Create:      time.Minute,
					Remove:      7 * time.Second,
					Setup:       5 * time.Minute,
					FirstOutput: 3 * time.Minute,
				},
			},
		},
		{
			name:   "the local branch format",
			local:  &config.LocalConfig{ProjectSlug: projectSlug, Repositories: repositories, Task: config.TaskConfig{BranchFormat: "feat/{{taskSlug}}"}},
			global: config.DefaultConfig(),
			want: drudger.Settings{
				ProjectSlug:           projectSlug,
				Repositories:          repositories,
				Env:                   drudger.EnvDockerSbx,
				Harness:               drudger.HarnessClaudeCode,
				MaxConcurrentDrudgers: 3,
				BranchFormat:          "feat/{{taskSlug}}",
				SandboxTimeouts: drudger.SandboxTimeouts{
					List:        30 * time.Second,
					Create:      10 * time.Minute,
					Remove:      2 * time.Minute,
					Setup:       15 * time.Minute,
					FirstOutput: time.Minute,
				},
			},
		},
		{
			name:   "the local environment variable names",
			local:  &config.LocalConfig{ProjectSlug: projectSlug, Repositories: repositories, Drudger: config.LocalDrudgerConfig{Env: []string{"NODE_AUTH_TOKEN"}}},
			global: config.DefaultConfig(),
			want: drudger.Settings{
				ProjectSlug:           projectSlug,
				Repositories:          repositories,
				Env:                   drudger.EnvDockerSbx,
				Harness:               drudger.HarnessClaudeCode,
				MaxConcurrentDrudgers: 3,
				EnvNames:              []string{"NODE_AUTH_TOKEN"},
				BranchFormat:          task.DefaultBranchFormat,
				SandboxTimeouts: drudger.SandboxTimeouts{
					List:        30 * time.Second,
					Create:      10 * time.Minute,
					Remove:      2 * time.Minute,
					Setup:       15 * time.Minute,
					FirstOutput: time.Minute,
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
				BranchFormat:          task.DefaultBranchFormat,
				SandboxTimeouts: drudger.SandboxTimeouts{
					List:        30 * time.Second,
					Create:      10 * time.Minute,
					Remove:      2 * time.Minute,
					Setup:       15 * time.Minute,
					FirstOutput: time.Minute,
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

func TestNewTaskService(t *testing.T) {
	const projectSlug = "drudge"

	draftGlobal := config.DefaultConfig()
	draftGlobal.Task.DefaultStatus = task.StatusDraft

	cases := []struct {
		name       string
		global     *config.GlobalConfig
		wantStatus task.TaskStatus
	}{
		{name: "the default global config", global: config.DefaultConfig(), wantStatus: task.StatusTodo},
		{name: "a global default status", global: draftGlobal, wantStatus: task.StatusDraft},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			t.Setenv("NO_COLOR", "1")
			t.Chdir(t.TempDir())
			if _, err := persistence.NewFileTaskRepository("other").CreateTask(task.CreateTaskDto{Title: "Other task", Status: task.StatusTodo, ProjectSlug: "other"}); err != nil {
				t.Fatal(err)
			}

			service := newTaskService(projectSlug, nil, testCase.global, newCommandPrinter())
			var created *task.Task
			var err error
			captureOutput(func() {
				created, err = service.CreateTask(task.CreateTaskDto{Title: "Add retry to uploader", ProjectSlug: projectSlug})
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			listed, err := service.ListTasks(projectSlug, task.ListTasksFilter{}, 1, 0)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(listed.Items) != 1 || listed.Items[0].Task.ID != created.ID {
				t.Fatalf("expected only task %s listed, got %+v", created.ID, listed.Items)
			}
			if got := listed.Items[0].Task.Status; got != testCase.wantStatus {
				t.Errorf("expected status %q, got %q", testCase.wantStatus, got)
			}
		})
	}
}
