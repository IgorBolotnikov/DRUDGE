package app

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/IgorBolotnikov/DRUDGE/internal/adapters/persistence"
	"github.com/IgorBolotnikov/DRUDGE/internal/common"
	"github.com/IgorBolotnikov/DRUDGE/internal/config"
	"github.com/IgorBolotnikov/DRUDGE/internal/drudger"
	"github.com/IgorBolotnikov/DRUDGE/internal/project"
	"github.com/IgorBolotnikov/DRUDGE/internal/task"
)

type silentProgress struct{}

func (silentProgress) Report(any) {}

func TestLoadConfigs(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Chdir(t.TempDir())

	_, err := LoadConfigs()
	if err == nil {
		t.Fatal("expected an error outside a project directory")
	}
	if !strings.Contains(err.Error(), "drg project init") {
		t.Errorf("expected the error to point at drg project init, got %q", err)
	}
}

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
			t.Chdir(t.TempDir())
			if _, err := persistence.NewFileTaskRepository("other").CreateTask(task.CreateTaskDto{Title: "Other task", Status: task.StatusTodo, ProjectSlug: "other"}); err != nil {
				t.Fatal(err)
			}

			service := NewTaskService(projectSlug, &Configs{Global: testCase.global}, silentProgress{})
			created, err := service.CreateTask(task.CreateTaskDto{Title: "Add retry to uploader", ProjectSlug: projectSlug})
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

func TestLoadGlobalConfig(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Chdir(t.TempDir())

	configs, err := LoadGlobalConfig()
	if err != nil {
		t.Fatalf("unexpected error outside a project directory: %v", err)
	}
	if configs.Local != nil {
		t.Errorf("expected no local config, got %+v", configs.Local)
	}
	if !reflect.DeepEqual(configs.Global, config.DefaultConfig()) {
		t.Errorf("expected the default global config, got %+v", configs.Global)
	}
}

func TestConfigsPageSizes(t *testing.T) {
	sizedGlobal := config.DefaultConfig()
	sizedGlobal.Task.PageSize = new(5)
	sizedGlobal.Drudger.PageSize = new(6)
	sizedGlobal.Project.PageSize = new(7)

	sizedLocal := &config.LocalConfig{}
	sizedLocal.Task.PageSize = new(0)
	sizedLocal.Drudger.PageSize = new(2)

	cases := []struct {
		name        string
		configs     *Configs
		wantTask    int
		wantDrudger int
		wantProject int
	}{
		{
			name:        "the default configs",
			configs:     &Configs{Local: &config.LocalConfig{}, Global: config.DefaultConfig()},
			wantTask:    20,
			wantDrudger: 20,
			wantProject: 20,
		},
		{
			name:        "sizes in the global config",
			configs:     &Configs{Local: &config.LocalConfig{}, Global: sizedGlobal},
			wantTask:    5,
			wantDrudger: 6,
			wantProject: 7,
		},
		{
			name:        "sizes in the local config",
			configs:     &Configs{Local: sizedLocal, Global: sizedGlobal},
			wantTask:    0,
			wantDrudger: 2,
			wantProject: 7,
		},
		{
			name:        "the global config alone",
			configs:     &Configs{Global: sizedGlobal},
			wantProject: 7,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if testCase.configs.Local != nil {
				if got := testCase.configs.TaskPageSize(); got != testCase.wantTask {
					t.Errorf("expected a task page size of %d, got %d", testCase.wantTask, got)
				}
				if got := testCase.configs.DrudgerPageSize(); got != testCase.wantDrudger {
					t.Errorf("expected a Drudger page size of %d, got %d", testCase.wantDrudger, got)
				}
			}
			if got := testCase.configs.ProjectPageSize(); got != testCase.wantProject {
				t.Errorf("expected a project page size of %d, got %d", testCase.wantProject, got)
			}
		})
	}
}

func TestNewReleaseService(t *testing.T) {
	if NewReleaseService(silentProgress{}) == nil {
		t.Fatal("expected a release service")
	}
}

func TestNewFiles(t *testing.T) {
	path := filepath.Join(t.TempDir(), "description.md")
	if err := os.WriteFile(path, []byte("Add retry to uploader"), common.DefaultFilePerm); err != nil {
		t.Fatal(err)
	}

	content, err := NewFiles().ReadFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(content) != "Add retry to uploader" {
		t.Errorf("expected the content of the file, got %q", content)
	}
}

func TestReadTuiConfigFile(t *testing.T) {
	cases := []struct {
		name string
		// content is the content of the TUI config file, nil for no file.
		content     *string
		wantContent []byte
		wantIsFound bool
	}{
		{name: "no file"},
		{name: "a file", content: new(`{"tabs": []}`), wantContent: []byte(`{"tabs": []}`), wantIsFound: true},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)
			path := common.TuiConfigPath(home)
			if testCase.content != nil {
				if err := common.EnsureDir(filepath.Dir(path)); err != nil {
					t.Fatal(err)
				}
				if err := common.WriteFile(path, *testCase.content); err != nil {
					t.Fatal(err)
				}
			}

			got, err := ReadTuiConfigFile()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			want := TuiConfigFile{Path: path, Content: testCase.wantContent, IsFound: testCase.wantIsFound}
			if !reflect.DeepEqual(got, want) {
				t.Errorf("expected %+v, got %+v", want, got)
			}
		})
	}
}

func TestReadTuiConfigFile_Unreadable(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	path := common.TuiConfigPath(home)
	if err := common.EnsureDir(path); err != nil {
		t.Fatal(err)
	}

	_, err := ReadTuiConfigFile()
	if err == nil || !strings.Contains(err.Error(), path) {
		t.Fatalf("expected an error naming %s, got %v", path, err)
	}
}
