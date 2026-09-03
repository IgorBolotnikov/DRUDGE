package config

import (
	"encoding/json"
	"os"
	"testing"

	"drudge/internal/common"
)

func setupHome(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", dir)
	t.Cleanup(func() { os.Setenv("HOME", origHome) })
	return dir
}

func writeConfig(t *testing.T, home string, cfg GlobalConfig) {
	t.Helper()
	if err := common.EnsureDir(common.DrudgeDir(home)); err != nil {
		t.Fatalf("could not create drudge dir: %v", err)
	}
	if err := common.WriteJSON(common.GlobalConfigPath(home), cfg); err != nil {
		t.Fatalf("could not write config: %v", err)
	}
}

func TestLoad_NoFile_ReturnsDefaults(t *testing.T) {
	setupHome(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.Runner.Env != defaultEnv {
		t.Errorf("Env = %q, want %q", cfg.Runner.Env, defaultEnv)
	}
	if cfg.Runner.Harness != defaultHarness {
		t.Errorf("Harness = %q, want %q", cfg.Runner.Harness, defaultHarness)
	}
}

func TestLoad_FullConfig_ReturnsLoadedValues(t *testing.T) {
	home := setupHome(t)
	writeConfig(t, home, GlobalConfig{
		Runner: RunnerConfig{
			Env:     "local",
			Harness: HarnessOpencode,
		},
	})

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.Runner.Env != "local" {
		t.Errorf("Env = %q, want %q", cfg.Runner.Env, "local")
	}
	if cfg.Runner.Harness != HarnessOpencode {
		t.Errorf("Harness = %q, want %q", cfg.Runner.Harness, HarnessOpencode)
	}
}

func TestLoad_MissingEnv_FallsBackToDefault(t *testing.T) {
	home := setupHome(t)
	writeConfig(t, home, GlobalConfig{
		Runner: RunnerConfig{
			Harness: HarnessOpencode,
		},
	})

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.Runner.Env != defaultEnv {
		t.Errorf("Env = %q, want default %q", cfg.Runner.Env, defaultEnv)
	}
	if cfg.Runner.Harness != HarnessOpencode {
		t.Errorf("Harness = %q, want %q", cfg.Runner.Harness, HarnessOpencode)
	}
}

func TestLoad_MissingHarness_FallsBackToDefault(t *testing.T) {
	home := setupHome(t)
	writeConfig(t, home, GlobalConfig{
		Runner: RunnerConfig{
			Env: "local",
		},
	})

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.Runner.Env != "local" {
		t.Errorf("Env = %q, want %q (should be untouched)", cfg.Runner.Env, "local")
	}
	if cfg.Runner.Harness != defaultHarness {
		t.Errorf("Harness = %q, want default %q", cfg.Runner.Harness, defaultHarness)
	}
}

func TestLoad_EmptyFile_ReturnsDefaults(t *testing.T) {
	home := setupHome(t)
	writeConfig(t, home, GlobalConfig{})

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.Runner.Env != defaultEnv {
		t.Errorf("Env = %q, want %q", cfg.Runner.Env, defaultEnv)
	}
	if cfg.Runner.Harness != defaultHarness {
		t.Errorf("Harness = %q, want %q", cfg.Runner.Harness, defaultHarness)
	}
}

func TestLoad_InvalidJSON_ReturnsError(t *testing.T) {
	home := setupHome(t)
	if err := common.EnsureDir(common.DrudgeDir(home)); err != nil {
		t.Fatalf("could not create drudge dir: %v", err)
	}
	if err := os.WriteFile(common.GlobalConfigPath(home), []byte("{not json"), common.DefaultFilePerm); err != nil {
		t.Fatalf("could not write config: %v", err)
	}

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Runner.Env != EnvDockerSbx {
		t.Errorf("Env = %q, want %q", cfg.Runner.Env, EnvDockerSbx)
	}
	if cfg.Runner.Harness != HarnessClaudeCode {
		t.Errorf("Harness = %q, want %q", cfg.Runner.Harness, HarnessClaudeCode)
	}
}

func TestMergeConfigs_KeepsLoadedValuesWhenSet(t *testing.T) {
	defaultCfg := DefaultConfig()
	loadedCfg := &GlobalConfig{
		Runner: RunnerConfig{
			Env:     "local",
			Harness: HarnessOpencode,
		},
	}

	merged := mergeConfigs(defaultCfg, loadedCfg)

	if merged.Runner.Env != "local" {
		t.Errorf("Env = %q, want %q", merged.Runner.Env, "local")
	}
	if merged.Runner.Harness != HarnessOpencode {
		t.Errorf("Harness = %q, want %q", merged.Runner.Harness, HarnessOpencode)
	}
}

func TestMergeConfigs_FillsEnvWhenEmpty(t *testing.T) {
	defaultCfg := DefaultConfig()
	loadedCfg := &GlobalConfig{
		Runner: RunnerConfig{
			Harness: HarnessOpencode,
		},
	}

	merged := mergeConfigs(defaultCfg, loadedCfg)

	if merged.Runner.Env != defaultCfg.Runner.Env {
		t.Errorf("Env = %q, want default %q", merged.Runner.Env, defaultCfg.Runner.Env)
	}
}

func TestSchema_ReturnsValidJSON(t *testing.T) {
	var schema map[string]any
	if err := json.Unmarshal(Schema(), &schema); err != nil {
		t.Fatalf("Schema() is not valid JSON: %v", err)
	}
}

func TestSchemaRef(t *testing.T) {
	if SchemaRef() != "./schema/config.json" {
		t.Errorf("SchemaRef() = %q, want %q", SchemaRef(), "./schema/config.json")
	}
}

func TestMergeConfigs_FillsHarnessWhenEmpty(t *testing.T) {
	defaultCfg := DefaultConfig()
	loadedCfg := &GlobalConfig{
		Runner: RunnerConfig{
			Env: "local",
		},
	}

	merged := mergeConfigs(defaultCfg, loadedCfg)

	if merged.Runner.Harness != defaultCfg.Runner.Harness {
		t.Errorf("Harness = %q, want default %q", merged.Runner.Harness, defaultCfg.Runner.Harness)
	}
	if merged.Runner.Env != "local" {
		t.Errorf("Env = %q, want %q (should be untouched)", merged.Runner.Env, "local")
	}
}

func TestLoad_MissingMaxConcurrentRunners_FallsBackToDefault(t *testing.T) {
	home := setupHome(t)
	writeConfig(t, home, GlobalConfig{
		Runner: RunnerConfig{
			Harness: HarnessOpencode,
		},
	})

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.Runner.MaxConcurrentRunners != defaultMaxConcurrentRunners {
		t.Errorf("MaxConcurrentRunners = %d, want default %d", cfg.Runner.MaxConcurrentRunners, defaultMaxConcurrentRunners)
	}
}

func TestLoad_RunnerOverrides_ReturnsLoadedValues(t *testing.T) {
	home := setupHome(t)
	writeConfig(t, home, GlobalConfig{
		Runner: RunnerConfig{
			PromptFile:           "impl.md",
			MaxConcurrentRunners: 7,
		},
	})

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.Runner.PromptFile != "impl.md" {
		t.Errorf("PromptFile = %q, want %q", cfg.Runner.PromptFile, "impl.md")
	}
	if cfg.Runner.MaxConcurrentRunners != 7 {
		t.Errorf("MaxConcurrentRunners = %d, want %d", cfg.Runner.MaxConcurrentRunners, 7)
	}
}

func TestLoad_NegativeMaxConcurrentRunners_ReturnsError(t *testing.T) {
	home := setupHome(t)
	if err := common.EnsureDir(common.DrudgeDir(home)); err != nil {
		t.Fatalf("could not create drudge dir: %v", err)
	}
	raw := []byte(`{"runner": {"maxConcurrentRunners": -1}}`)
	if err := os.WriteFile(common.GlobalConfigPath(home), raw, common.DefaultFilePerm); err != nil {
		t.Fatalf("could not write config: %v", err)
	}

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for a negative runner limit")
	}
}

func TestLoad_PromptFileOutsidePromptsDir_ReturnsError(t *testing.T) {
	tests := []struct {
		name       string
		promptFile string
	}{
		{name: "subdirectory", promptFile: "sub/impl.md"},
		{name: "parent directory", promptFile: "../impl.md"},
		{name: "absolute path", promptFile: "/etc/impl.md"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			home := setupHome(t)
			writeConfig(t, home, GlobalConfig{
				Runner: RunnerConfig{PromptFile: test.promptFile},
			})

			_, err := Load()
			if err == nil {
				t.Fatalf("expected an error for prompt file %q", test.promptFile)
			}
		})
	}
}
