package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/IgorBolotnikov/DRUDGE/internal/common"
	"github.com/IgorBolotnikov/DRUDGE/internal/task"
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
		name            string
		raw             string
		shouldWriteFile bool
		wantErr         bool
		wantSchema      string
		wantSlug        string
		wantPrompt      string
		wantDrudgers    int
		wantRepos       []Repository
		wantTaskStatus  task.TaskStatus
	}{
		{
			name:            "no file",
			shouldWriteFile: false,
			wantErr:         true,
		},
		{
			name:            "slug only",
			shouldWriteFile: true,
			raw:             `{"projectSlug": "test-project"}`,
			wantSlug:        "test-project",
		},
		{
			name:            "slug and schema",
			shouldWriteFile: true,
			raw:             `{"$schema": "/home/user/.drudge/schema/local-config.json", "projectSlug": "test-project"}`,
			wantSchema:      "/home/user/.drudge/schema/local-config.json",
			wantSlug:        "test-project",
		},
		{
			name:            "all fields",
			shouldWriteFile: true,
			raw:             `{"projectSlug": "test-project", "promptFile": "impl.md", "maxConcurrentDrudgers": 5}`,
			wantSlug:        "test-project",
			wantPrompt:      "impl.md",
			wantDrudgers:    5,
		},
		{
			name:            "missing slug",
			shouldWriteFile: true,
			raw:             `{"promptFile": "impl.md"}`,
			wantErr:         true,
		},
		{
			name:            "empty slug",
			shouldWriteFile: true,
			raw:             `{"projectSlug": ""}`,
			wantErr:         true,
		},
		{
			name:            "invalid json",
			shouldWriteFile: true,
			raw:             `{not json`,
			wantErr:         true,
		},
		{
			name:            "prompt file in a subdirectory",
			shouldWriteFile: true,
			raw:             `{"projectSlug": "test-project", "promptFile": "sub/impl.md"}`,
			wantErr:         true,
		},
		{
			name:            "prompt file escaping the prompts directory",
			shouldWriteFile: true,
			raw:             `{"projectSlug": "test-project", "promptFile": "../impl.md"}`,
			wantErr:         true,
		},
		{
			name:            "negative Drudger limit",
			shouldWriteFile: true,
			raw:             `{"projectSlug": "test-project", "maxConcurrentDrudgers": -1}`,
			wantErr:         true,
		},
		{
			name:            "no repositories key",
			shouldWriteFile: true,
			raw:             `{"projectSlug": "test-project"}`,
			wantSlug:        "test-project",
			wantRepos:       nil,
		},
		{
			name:            "one repository",
			shouldWriteFile: true,
			raw:             `{"projectSlug": "test-project", "repositories": [{"path": "."}]}`,
			wantSlug:        "test-project",
			wantRepos:       []Repository{{Path: "."}},
		},
		{
			name:            "repositories with a default branch",
			shouldWriteFile: true,
			raw:             `{"projectSlug": "test-project", "repositories": [{"path": "api", "defaultBranch": "trunk"}, {"path": "ui"}]}`,
			wantSlug:        "test-project",
			wantRepos:       []Repository{{Path: "api", DefaultBranch: "trunk"}, {Path: "ui"}},
		},
		{
			name:            "repository with no path",
			shouldWriteFile: true,
			raw:             `{"projectSlug": "test-project", "repositories": [{"defaultBranch": "main"}]}`,
			wantErr:         true,
		},
		{
			name:            "repository path outside the project directory",
			shouldWriteFile: true,
			raw:             `{"projectSlug": "test-project", "repositories": [{"path": "../elsewhere"}]}`,
			wantErr:         true,
		},
		{
			name:            "draft default task status",
			shouldWriteFile: true,
			raw:             `{"projectSlug": "test-project", "task": {"defaultStatus": "draft"}}`,
			wantSlug:        "test-project",
			wantTaskStatus:  task.StatusDraft,
		},
		{
			name:            "todo default task status",
			shouldWriteFile: true,
			raw:             `{"projectSlug": "test-project", "task": {"defaultStatus": "todo"}}`,
			wantSlug:        "test-project",
			wantTaskStatus:  task.StatusTodo,
		},
		{
			name:            "default task status a new task cannot start in",
			shouldWriteFile: true,
			raw:             `{"projectSlug": "test-project", "task": {"defaultStatus": "done"}}`,
			wantErr:         true,
		},
		{
			name:            "absolute repository path",
			shouldWriteFile: true,
			raw:             `{"projectSlug": "test-project", "repositories": [{"path": "/srv/elsewhere"}]}`,
			wantErr:         true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupLocalDir(t)
			if test.shouldWriteFile {
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

			if cfg.Schema != test.wantSchema {
				t.Errorf("Schema = %q, want %q", cfg.Schema, test.wantSchema)
			}
			if cfg.ProjectSlug != test.wantSlug {
				t.Errorf("ProjectSlug = %q, want %q", cfg.ProjectSlug, test.wantSlug)
			}
			if cfg.PromptFile != test.wantPrompt {
				t.Errorf("PromptFile = %q, want %q", cfg.PromptFile, test.wantPrompt)
			}
			if cfg.MaxConcurrentDrudgers != test.wantDrudgers {
				t.Errorf("MaxConcurrentDrudgers = %d, want %d", cfg.MaxConcurrentDrudgers, test.wantDrudgers)
			}
			if !reflect.DeepEqual(cfg.Repositories, test.wantRepos) {
				t.Errorf("Repositories = %+v, want %+v", cfg.Repositories, test.wantRepos)
			}
			if cfg.Task.DefaultStatus != test.wantTaskStatus {
				t.Errorf("Task.DefaultStatus = %q, want %q", cfg.Task.DefaultStatus, test.wantTaskStatus)
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

func TestLoadLocal_InvalidDefaultTaskStatus_ErrorNamesFileKeyAndValues(t *testing.T) {
	setupLocalDir(t)
	writeLocalConfig(t, `{"projectSlug": "test-project", "task": {"defaultStatus": "someday"}}`)

	_, err := LoadLocal()
	if err == nil {
		t.Fatal("expected an error")
	}
	for _, fragment := range []string{common.LocalConfigPath(), DefaultTaskStatusKey, `"someday"`, "draft, todo"} {
		if !strings.Contains(err.Error(), fragment) {
			t.Errorf("error = %q, want it to name %q", err, fragment)
		}
	}
}

func TestSave_RoundTrips(t *testing.T) {
	setupLocalDir(t)

	cfg := LocalConfig{
		Schema:                "/home/user/.drudge/schema/local-config.json",
		ProjectSlug:           "test-project",
		PromptFile:            "impl.md",
		MaxConcurrentDrudgers: 5,
		Task:                  TaskConfig{DefaultStatus: task.StatusTodo},
		Repositories:          []Repository{{Path: "api", DefaultBranch: "trunk"}, {Path: "ui"}},
	}
	if err := cfg.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := LoadLocal()
	if err != nil {
		t.Fatalf("LoadLocal: %v", err)
	}
	if !reflect.DeepEqual(*loaded, cfg) {
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

func TestLocalSchema_DescribesEveryLocalConfigKey(t *testing.T) {
	var schema map[string]any
	if err := json.Unmarshal(LocalSchema(), &schema); err != nil {
		t.Fatalf("LocalSchema() is not valid JSON: %v", err)
	}
	assertSchemaDescribes(t, schema, reflect.TypeFor[LocalConfig](), "")
}

// assertSchemaDescribes fails for every JSON key of structType, and of the
// structs it nests, that has no entry in the properties of schema.
func assertSchemaDescribes(t *testing.T, schema map[string]any, structType reflect.Type, keyPrefix string) {
	t.Helper()
	properties, ok := schema["properties"].(map[string]any)
	if !ok {
		t.Errorf("schema of %q has no properties", keyPrefix)
		return
	}
	for index := range structType.NumField() {
		field := structType.Field(index)
		key, _, _ := strings.Cut(field.Tag.Get("json"), ",")
		keyPath := keyPrefix + key
		property, ok := properties[key].(map[string]any)
		if !ok {
			t.Errorf("schema has no entry for %q", keyPath)
			continue
		}
		switch {
		case field.Type.Kind() == reflect.Struct:
			assertSchemaDescribes(t, property, field.Type, keyPath+".")
		case field.Type.Kind() == reflect.Slice && field.Type.Elem().Kind() == reflect.Struct:
			items, ok := property["items"].(map[string]any)
			if !ok {
				t.Errorf("schema of %q has no items", keyPath)
				continue
			}
			assertSchemaDescribes(t, items, field.Type.Elem(), keyPath+"[].")
		}
	}
}

func TestLocalSchemaRef(t *testing.T) {
	want := filepath.Join("/home/user", ".drudge", "schema", "local-config.json")
	if got := LocalSchemaRef("/home/user"); got != want {
		t.Errorf("LocalSchemaRef = %q, want %q", got, want)
	}
}

func TestResolvePromptPath(t *testing.T) {
	home := setupHome(t)

	tests := []struct {
		name   string
		local  *LocalConfig
		global *GlobalConfig
		want   string
	}{
		{
			name:   "local wins over global",
			local:  &LocalConfig{PromptFile: "local.md"},
			global: &GlobalConfig{Drudger: DrudgerConfig{PromptFile: "global.md"}},
			want:   filepath.Join(common.LocalPromptsDir(), "local.md"),
		},
		{
			name:   "falls back to global",
			local:  &LocalConfig{},
			global: &GlobalConfig{Drudger: DrudgerConfig{PromptFile: "global.md"}},
			want:   filepath.Join(common.PromptsDir(home), "global.md"),
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
			got, err := ResolvePromptPath(test.local, test.global)
			if err != nil {
				t.Fatalf("ResolvePromptPath: %v", err)
			}
			if got != test.want {
				t.Errorf("ResolvePromptPath = %q, want %q", got, test.want)
			}
		})
	}
}

func TestResolveMaxConcurrentDrudgers(t *testing.T) {
	tests := []struct {
		name   string
		local  *LocalConfig
		global *GlobalConfig
		want   int
	}{
		{
			name:   "local wins over global",
			local:  &LocalConfig{MaxConcurrentDrudgers: 5},
			global: &GlobalConfig{Drudger: DrudgerConfig{MaxConcurrentDrudgers: 7}},
			want:   5,
		},
		{
			name:   "falls back to global",
			local:  &LocalConfig{},
			global: &GlobalConfig{Drudger: DrudgerConfig{MaxConcurrentDrudgers: 7}},
			want:   7,
		},
		{
			name:   "falls back to default",
			local:  &LocalConfig{},
			global: &GlobalConfig{},
			want:   defaultMaxConcurrentDrudgers,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := ResolveMaxConcurrentDrudgers(test.local, test.global)
			if got != test.want {
				t.Errorf("ResolveMaxConcurrentDrudgers = %d, want %d", got, test.want)
			}
		})
	}
}

func TestResolveDefaultTaskStatus(t *testing.T) {
	tests := []struct {
		name   string
		local  *LocalConfig
		global *GlobalConfig
		want   task.TaskStatus
	}{
		{
			name:   "local wins over global",
			local:  &LocalConfig{Task: TaskConfig{DefaultStatus: task.StatusDraft}},
			global: &GlobalConfig{Task: TaskConfig{DefaultStatus: task.StatusTodo}},
			want:   task.StatusDraft,
		},
		{
			name:   "falls back to global",
			local:  &LocalConfig{},
			global: &GlobalConfig{Task: TaskConfig{DefaultStatus: task.StatusTodo}},
			want:   task.StatusTodo,
		},
		{
			name:   "falls back to draft",
			local:  &LocalConfig{},
			global: &GlobalConfig{},
			want:   task.StatusDraft,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := ResolveDefaultTaskStatus(test.local, test.global)
			if got != test.want {
				t.Errorf("ResolveDefaultTaskStatus = %q, want %q", got, test.want)
			}
		})
	}
}
