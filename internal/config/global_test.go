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

	if cfg.Drudger.Env != defaultEnv {
		t.Errorf("Env = %q, want %q", cfg.Drudger.Env, defaultEnv)
	}
	if cfg.Drudger.Harness != defaultHarness {
		t.Errorf("Harness = %q, want %q", cfg.Drudger.Harness, defaultHarness)
	}
}

func TestLoad_FullConfig_ReturnsLoadedValues(t *testing.T) {
	home := setupHome(t)
	writeConfig(t, home, GlobalConfig{
		Drudger: DrudgerConfig{
			Env:     "local",
			Harness: HarnessOpencode,
		},
	})

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.Drudger.Env != "local" {
		t.Errorf("Env = %q, want %q", cfg.Drudger.Env, "local")
	}
	if cfg.Drudger.Harness != HarnessOpencode {
		t.Errorf("Harness = %q, want %q", cfg.Drudger.Harness, HarnessOpencode)
	}
}

func TestLoad_MissingEnv_FallsBackToDefault(t *testing.T) {
	home := setupHome(t)
	writeConfig(t, home, GlobalConfig{
		Drudger: DrudgerConfig{
			Harness: HarnessOpencode,
		},
	})

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.Drudger.Env != defaultEnv {
		t.Errorf("Env = %q, want default %q", cfg.Drudger.Env, defaultEnv)
	}
	if cfg.Drudger.Harness != HarnessOpencode {
		t.Errorf("Harness = %q, want %q", cfg.Drudger.Harness, HarnessOpencode)
	}
}

func TestLoad_MissingHarness_FallsBackToDefault(t *testing.T) {
	home := setupHome(t)
	writeConfig(t, home, GlobalConfig{
		Drudger: DrudgerConfig{
			Env: "local",
		},
	})

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.Drudger.Env != "local" {
		t.Errorf("Env = %q, want %q (should be untouched)", cfg.Drudger.Env, "local")
	}
	if cfg.Drudger.Harness != defaultHarness {
		t.Errorf("Harness = %q, want default %q", cfg.Drudger.Harness, defaultHarness)
	}
}

func TestLoad_EmptyFile_ReturnsDefaults(t *testing.T) {
	home := setupHome(t)
	writeConfig(t, home, GlobalConfig{})

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.Drudger.Env != defaultEnv {
		t.Errorf("Env = %q, want %q", cfg.Drudger.Env, defaultEnv)
	}
	if cfg.Drudger.Harness != defaultHarness {
		t.Errorf("Harness = %q, want %q", cfg.Drudger.Harness, defaultHarness)
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

	if cfg.Drudger.Env != EnvDockerSbx {
		t.Errorf("Env = %q, want %q", cfg.Drudger.Env, EnvDockerSbx)
	}
	if cfg.Drudger.Harness != HarnessClaudeCode {
		t.Errorf("Harness = %q, want %q", cfg.Drudger.Harness, HarnessClaudeCode)
	}
}

func TestMergeConfigs_KeepsLoadedValuesWhenSet(t *testing.T) {
	defaultCfg := DefaultConfig()
	loadedCfg := &GlobalConfig{
		Drudger: DrudgerConfig{
			Env:     "local",
			Harness: HarnessOpencode,
		},
	}

	merged := mergeConfigs(defaultCfg, loadedCfg)

	if merged.Drudger.Env != "local" {
		t.Errorf("Env = %q, want %q", merged.Drudger.Env, "local")
	}
	if merged.Drudger.Harness != HarnessOpencode {
		t.Errorf("Harness = %q, want %q", merged.Drudger.Harness, HarnessOpencode)
	}
}

func TestMergeConfigs_FillsEnvWhenEmpty(t *testing.T) {
	defaultCfg := DefaultConfig()
	loadedCfg := &GlobalConfig{
		Drudger: DrudgerConfig{
			Harness: HarnessOpencode,
		},
	}

	merged := mergeConfigs(defaultCfg, loadedCfg)

	if merged.Drudger.Env != defaultCfg.Drudger.Env {
		t.Errorf("Env = %q, want default %q", merged.Drudger.Env, defaultCfg.Drudger.Env)
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
		Drudger: DrudgerConfig{
			Env: "local",
		},
	}

	merged := mergeConfigs(defaultCfg, loadedCfg)

	if merged.Drudger.Harness != defaultCfg.Drudger.Harness {
		t.Errorf("Harness = %q, want default %q", merged.Drudger.Harness, defaultCfg.Drudger.Harness)
	}
	if merged.Drudger.Env != "local" {
		t.Errorf("Env = %q, want %q (should be untouched)", merged.Drudger.Env, "local")
	}
}

func TestLoad_MissingMaxConcurrentDrudgers_FallsBackToDefault(t *testing.T) {
	home := setupHome(t)
	writeConfig(t, home, GlobalConfig{
		Drudger: DrudgerConfig{
			Harness: HarnessOpencode,
		},
	})

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.Drudger.MaxConcurrentDrudgers != defaultMaxConcurrentDrudgers {
		t.Errorf("MaxConcurrentDrudgers = %d, want default %d", cfg.Drudger.MaxConcurrentDrudgers, defaultMaxConcurrentDrudgers)
	}
}

func TestLoad_DrudgerOverrides_ReturnsLoadedValues(t *testing.T) {
	home := setupHome(t)
	writeConfig(t, home, GlobalConfig{
		Drudger: DrudgerConfig{
			PromptFile:            "impl.md",
			MaxConcurrentDrudgers: 7,
		},
	})

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.Drudger.PromptFile != "impl.md" {
		t.Errorf("PromptFile = %q, want %q", cfg.Drudger.PromptFile, "impl.md")
	}
	if cfg.Drudger.MaxConcurrentDrudgers != 7 {
		t.Errorf("MaxConcurrentDrudgers = %d, want %d", cfg.Drudger.MaxConcurrentDrudgers, 7)
	}
}

func TestLoad_NegativeMaxConcurrentDrudgers_ReturnsError(t *testing.T) {
	home := setupHome(t)
	if err := common.EnsureDir(common.DrudgeDir(home)); err != nil {
		t.Fatalf("could not create drudge dir: %v", err)
	}
	raw := []byte(`{"drudger": {"maxConcurrentDrudgers": -1}}`)
	if err := os.WriteFile(common.GlobalConfigPath(home), raw, common.DefaultFilePerm); err != nil {
		t.Fatalf("could not write config: %v", err)
	}

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for a negative Drudger limit")
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
				Drudger: DrudgerConfig{PromptFile: test.promptFile},
			})

			_, err := Load()
			if err == nil {
				t.Fatalf("expected an error for prompt file %q", test.promptFile)
			}
		})
	}
}
