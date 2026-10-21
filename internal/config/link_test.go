package config

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/IgorBolotnikov/DRUDGE/internal/common"
	"github.com/IgorBolotnikov/DRUDGE/internal/project"
	"github.com/IgorBolotnikov/DRUDGE/internal/remote"
)

func TestLocalConfigLinker_LinkDirectory(t *testing.T) {
	tests := []struct {
		name         string
		slug         string
		repositories []project.Repository
		envNames     []string
		overrides    common.Answers
		// wantMaxConcurrent, wantTask and wantRemote are what overrides sets.
		wantMaxConcurrent int
		wantTask          TaskConfig
		wantRemote        RemoteConfig
		wantErr           bool
	}{
		{
			name:         "a directory that is a repository",
			slug:         "test-project",
			repositories: []project.Repository{{Path: "."}},
		},
		{
			name:         "a directory holding repositories",
			slug:         "my-cool-app",
			repositories: []project.Repository{{Path: "api", DefaultBranch: "trunk"}, {Path: "ui"}},
		},
		{
			name:         "a repository that needs environment variables",
			slug:         "test-project",
			repositories: []project.Repository{{Path: "."}},
			envNames:     []string{"NPM_TOKEN"},
		},
		{
			name:         "answers that override the global config",
			slug:         "test-project",
			repositories: []project.Repository{{Path: "."}},
			overrides: common.Answers{
				maxConcurrentDrudgersKey: "5",
				branchFormatKey:          "fix/{{taskSlug}}",
				RemoteProviderKey:        "github",
				PullRequestsEnabledKey:   common.AnswerYes,
				PullRequestsDraftKey:     common.AnswerNo,
			},
			wantMaxConcurrent: 5,
			wantTask:          TaskConfig{BranchFormat: "fix/{{taskSlug}}"},
			wantRemote: RemoteConfig{
				Provider:     remote.ProviderGitHub,
				PullRequests: PullRequestsConfig{IsEnabled: new(true), IsDraft: new(false)},
			},
		},
		{
			name:         "an answer for a key the local config cannot take",
			slug:         "test-project",
			repositories: []project.Repository{{Path: "."}},
			overrides:    common.Answers{DrudgerHarnessKey: "opencode"},
			wantErr:      true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupLocalDir(t)
			home := "/home/user"

			err := NewLocalConfigLinker(home).LinkDirectory(test.slug, test.repositories, test.envNames, test.overrides)
			if test.wantErr {
				if err == nil {
					t.Fatal("expected LinkDirectory to refuse the answers")
				}
				if isPresent, _ := common.Exists(common.LocalConfigPath()); isPresent {
					t.Error("expected a refused link to write no local config")
				}
				return
			}
			if err != nil {
				t.Fatalf("LinkDirectory: %v", err)
			}

			loaded, err := LoadLocal()
			if err != nil {
				t.Fatalf("LoadLocal: %v", err)
			}
			want := LocalConfig{
				Schema:                filepath.Join(home, ".drudge", "schema", "local-config.json"),
				ProjectSlug:           test.slug,
				MaxConcurrentDrudgers: test.wantMaxConcurrent,
				Drudger:               LocalDrudgerConfig{Env: test.envNames},
				Task:                  test.wantTask,
				Remote:                test.wantRemote,
				Repositories:          test.repositories,
			}
			if !reflect.DeepEqual(*loaded, want) {
				t.Errorf("loaded = %+v, want %+v", *loaded, want)
			}
		})
	}
}

func TestLocalConfigLinker_LinkedSlug(t *testing.T) {
	tests := []struct {
		name           string
		raw            string
		hasLocalConfig bool
		wantSlug       string
		wantIsLinked   bool
		wantErr        bool
	}{
		{name: "a directory with no local config"},
		{name: "a project directory", raw: `{"projectSlug": "drudge"}`, hasLocalConfig: true, wantSlug: "drudge", wantIsLinked: true},
		{name: "a local config with no slug", raw: `{}`, hasLocalConfig: true, wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupLocalDir(t)
			if test.hasLocalConfig {
				writeLocalConfig(t, test.raw)
			}

			slug, isLinked, err := NewLocalConfigLinker("/home/user").LinkedSlug()

			if test.wantErr {
				if err == nil {
					t.Fatal("expected an error")
				}
				return
			}
			if err != nil {
				t.Fatalf("LinkedSlug: %v", err)
			}
			if slug != test.wantSlug || isLinked != test.wantIsLinked {
				t.Errorf("LinkedSlug = %q, %v, want %q, %v", slug, isLinked, test.wantSlug, test.wantIsLinked)
			}
		})
	}
}

func TestLocalConfigLinker_RecordedSetup(t *testing.T) {
	tests := []struct {
		name             string
		raw              string
		hasLocalConfig   bool
		wantRepositories []project.Repository
		wantEnvNames     []string
		wantErr          bool
	}{
		{name: "a directory with no local config", wantErr: true},
		{name: "a local config with no slug", raw: `{}`, hasLocalConfig: true, wantErr: true},
		{
			name:             "a project directory",
			raw:              `{"projectSlug": "drudge", "drudger": {"env": ["NPM_TOKEN"]}, "repositories": [{"path": "api", "setup": ["npm ci"]}]}`,
			hasLocalConfig:   true,
			wantRepositories: []project.Repository{{Path: "api", Setup: []string{"npm ci"}}},
			wantEnvNames:     []string{"NPM_TOKEN"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupLocalDir(t)
			if test.hasLocalConfig {
				writeLocalConfig(t, test.raw)
			}

			repositories, envNames, err := NewLocalConfigLinker("/home/user").RecordedSetup()

			if test.wantErr {
				if err == nil {
					t.Fatal("expected an error")
				}
				return
			}
			if err != nil {
				t.Fatalf("RecordedSetup: %v", err)
			}
			if !reflect.DeepEqual(repositories, test.wantRepositories) || !reflect.DeepEqual(envNames, test.wantEnvNames) {
				t.Errorf("RecordedSetup = %+v, %q, want %+v, %q", repositories, envNames, test.wantRepositories, test.wantEnvNames)
			}
		})
	}
}

func TestLocalConfigLinker_RecordSetup(t *testing.T) {
	tests := []struct {
		name         string
		repositories []project.Repository
		setups       map[string]project.DetectedSetup
		// listedEnv is the Drudger env before the setup is recorded.
		listedEnv []string
		envNames  []string
		want      []project.Repository
		wantEnv   []string
	}{
		{
			name:         "a missing setup is filled",
			repositories: []project.Repository{{Path: "api", DefaultBranch: "trunk"}},
			setups:       map[string]project.DetectedSetup{"api": {Setup: []string{"npm ci"}}},
			want:         []project.Repository{{Path: "api", DefaultBranch: "trunk", Setup: []string{"npm ci"}}},
		},
		{
			name:         "an empty setup is kept as an empty list",
			repositories: []project.Repository{{Path: "api"}},
			setups:       map[string]project.DetectedSetup{"api": {Setup: []string{}, Copy: []string{}}},
			want:         []project.Repository{{Path: "api", Setup: []string{}, Copy: []string{}}},
		},
		{
			name:         "a repository setups does not name is left alone",
			repositories: []project.Repository{{Path: "api", Setup: []string{"make deps"}}, {Path: "ui"}},
			setups:       map[string]project.DetectedSetup{"ui": {Setup: []string{"npm ci"}}},
			want:         []project.Repository{{Path: "api", Setup: []string{"make deps"}}, {Path: "ui", Setup: []string{"npm ci"}}},
		},
		{
			name:         "copy paths replace the ones there",
			repositories: []project.Repository{{Path: "api", Copy: []string{"secrets.json"}}},
			setups:       map[string]project.DetectedSetup{"api": {Setup: []string{}, Copy: []string{".env"}}},
			want:         []project.Repository{{Path: "api", Setup: []string{}, Copy: []string{".env"}}},
		},
		{
			name:         "no copy paths keep the ones there",
			repositories: []project.Repository{{Path: "api", Copy: []string{"secrets.json"}}},
			setups:       map[string]project.DetectedSetup{"api": {Setup: []string{"npm ci"}}},
			want:         []project.Repository{{Path: "api", Setup: []string{"npm ci"}, Copy: []string{"secrets.json"}}},
		},
		{
			name:         "env names are added after the listed ones",
			repositories: []project.Repository{{Path: "api", Setup: []string{"make deps"}}},
			listedEnv:    []string{"GITHUB_TOKEN"},
			envNames:     []string{"NPM_TOKEN", "GITHUB_TOKEN"},
			want:         []project.Repository{{Path: "api", Setup: []string{"make deps"}}},
			wantEnv:      []string{"GITHUB_TOKEN", "NPM_TOKEN"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupLocalDir(t)
			linker := NewLocalConfigLinker("/home/user")
			if err := linker.LinkDirectory("demo", test.repositories, test.listedEnv, nil); err != nil {
				t.Fatalf("LinkDirectory: %v", err)
			}

			if err := linker.RecordSetup(test.setups, test.envNames); err != nil {
				t.Fatalf("RecordSetup: %v", err)
			}

			loaded, err := LoadLocal()
			if err != nil {
				t.Fatalf("LoadLocal: %v", err)
			}
			if !reflect.DeepEqual(loaded.Repositories, test.want) {
				t.Errorf("repositories = %+v, want %+v", loaded.Repositories, test.want)
			}
			if !reflect.DeepEqual(loaded.Drudger.Env, test.wantEnv) {
				t.Errorf("env = %q, want %q", loaded.Drudger.Env, test.wantEnv)
			}
		})
	}
}
