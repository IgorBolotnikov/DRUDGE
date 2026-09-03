package config

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"drudge/internal/common"
)

// setupLocalDir chdirs into a temp dir so the relative drudge paths the
// local config uses resolve inside it.
func setupLocalDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	origCwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("Chdir: %v", err)
	}
	t.Cleanup(func() { os.Chdir(origCwd) })
	return dir
}

func writeLocalConfig(t *testing.T, raw string) {
	t.Helper()
	if err := common.EnsureDir(common.DotDrudgeDirName); err != nil {
		t.Fatalf("could not create local drudge dir: %v", err)
	}
	if err := os.WriteFile(common.LocalConfigPath(), []byte(raw), common.DefaultFilePerm); err != nil {
		t.Fatalf("could not write local config: %v", err)
	}
}

func TestLoadLocal(t *testing.T) {
	tests := []struct {
		name        string
		raw         string
		writeFile   bool
		wantErr     bool
		wantSlug    string
		wantPrompt  string
		wantRunners int
	}{
		{
			name:      "no file",
			writeFile: false,
			wantErr:   true,
		},
		{
			name:      "slug only",
			writeFile: true,
			raw:       `{"projectSlug": "test-project"}`,
			wantSlug:  "test-project",
		},
		{
			name:        "all fields",
			writeFile:   true,
			raw:         `{"projectSlug": "test-project", "promptFile": "impl.md", "maxConcurrentRunners": 5}`,
			wantSlug:    "test-project",
			wantPrompt:  "impl.md",
			wantRunners: 5,
		},
		{
			name:      "missing slug",
			writeFile: true,
			raw:       `{"promptFile": "impl.md"}`,
			wantErr:   true,
		},
		{
			name:      "empty slug",
			writeFile: true,
			raw:       `{"projectSlug": ""}`,
			wantErr:   true,
		},
		{
			name:      "invalid json",
			writeFile: true,
			raw:       `{not json`,
			wantErr:   true,
		},
		{
			name:      "negative runner limit",
			writeFile: true,
			raw:       `{"projectSlug": "test-project", "maxConcurrentRunners": -1}`,
			wantErr:   true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupLocalDir(t)
			if test.writeFile {
				writeLocalConfig(t, test.raw)
			}

			cfg, err := LoadLocal()
			if test.wantErr {
				if err == nil {
					t.Fatal("expected an error")
				}
				return
			}
			if err != nil {
				t.Fatalf("LoadLocal: %v", err)
			}

			if cfg.ProjectSlug != test.wantSlug {
				t.Errorf("ProjectSlug = %q, want %q", cfg.ProjectSlug, test.wantSlug)
			}
			if cfg.PromptFile != test.wantPrompt {
				t.Errorf("PromptFile = %q, want %q", cfg.PromptFile, test.wantPrompt)
			}
			if cfg.MaxConcurrentRunners != test.wantRunners {
				t.Errorf("MaxConcurrentRunners = %d, want %d", cfg.MaxConcurrentRunners, test.wantRunners)
			}
		})
	}
}

func TestLoadLocal_NoFile_ErrorNamesPath(t *testing.T) {
	setupLocalDir(t)

	_, err := LoadLocal()
	if err == nil {
		t.Fatal("expected an error")
	}
	if !strings.Contains(err.Error(), common.LocalConfigPath()) {
		t.Errorf("error = %q, want it to name %q", err, common.LocalConfigPath())
	}
}

func TestSave_RoundTrips(t *testing.T) {
	setupLocalDir(t)

	cfg := LocalConfig{
		ProjectSlug:          "test-project",
		PromptFile:           "impl.md",
		MaxConcurrentRunners: 5,
	}
	if err := cfg.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := LoadLocal()
	if err != nil {
		t.Fatalf("LoadLocal: %v", err)
	}
	if *loaded != cfg {
		t.Errorf("loaded = %+v, want %+v", *loaded, cfg)
	}
}

func TestSave_OmitsUnsetOverrides(t *testing.T) {
	setupLocalDir(t)

	cfg := LocalConfig{ProjectSlug: "test-project"}
	if err := cfg.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	data, err := os.ReadFile(common.LocalConfigPath())
	if err != nil {
		t.Fatalf("could not read local config: %v", err)
	}

	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("could not parse local config: %v", err)
	}
	if len(raw) != 1 {
		t.Errorf("config keys = %v, want only %q", raw, projectSlugKey)
	}
	if raw[projectSlugKey] != "test-project" {
		t.Errorf("%s = %v, want %q", projectSlugKey, raw[projectSlugKey], "test-project")
	}
}

func TestResolvePromptFile(t *testing.T) {
	tests := []struct {
		name   string
		local  *LocalConfig
		global *GlobalConfig
		want   string
	}{
		{
			name:   "local wins over global",
			local:  &LocalConfig{PromptFile: "local.md"},
			global: &GlobalConfig{Runner: RunnerConfig{PromptFile: "global.md"}},
			want:   "local.md",
		},
		{
			name:   "falls back to global",
			local:  &LocalConfig{},
			global: &GlobalConfig{Runner: RunnerConfig{PromptFile: "global.md"}},
			want:   "global.md",
		},
		{
			name:   "neither set",
			local:  &LocalConfig{},
			global: &GlobalConfig{},
			want:   "",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := ResolvePromptFile(test.local, test.global)
			if got != test.want {
				t.Errorf("ResolvePromptFile = %q, want %q", got, test.want)
			}
		})
	}
}

func TestResolveMaxConcurrentRunners(t *testing.T) {
	tests := []struct {
		name   string
		local  *LocalConfig
		global *GlobalConfig
		want   int
	}{
		{
			name:   "local wins over global",
			local:  &LocalConfig{MaxConcurrentRunners: 5},
			global: &GlobalConfig{Runner: RunnerConfig{MaxConcurrentRunners: 7}},
			want:   5,
		},
		{
			name:   "falls back to global",
			local:  &LocalConfig{},
			global: &GlobalConfig{Runner: RunnerConfig{MaxConcurrentRunners: 7}},
			want:   7,
		},
		{
			name:   "falls back to default",
			local:  &LocalConfig{},
			global: &GlobalConfig{},
			want:   defaultMaxConcurrentRunners,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := ResolveMaxConcurrentRunners(test.local, test.global)
			if got != test.want {
				t.Errorf("ResolveMaxConcurrentRunners = %d, want %d", got, test.want)
			}
		})
	}
}
