// Package config holds all configs which DRUDGE creates
package config

import (
	"fmt"

	"drudge/internal/common"
)

type Env string

const (
	EnvDockerSbx Env = "docker-sbx"
)

type Harness string

const (
	HarnessClaudeCode Harness = "claude-code"
	HarnessOpencode   Harness = "opencode"
)

const (
	defaultEnv                  = EnvDockerSbx
	defaultHarness              = HarnessClaudeCode
	defaultMaxConcurrentRunners = 3
)

// JSON keys, named in error messages so they match what a user writes in a config file.
const (
	projectSlugKey          = "projectSlug"
	maxConcurrentRunnersKey = "maxConcurrentRunners"
)

// schemaRef is the $schema reference path in config.json.
const schemaRef = "./schema/config.json"

// SchemaRef returns the $schema reference path for config.json.
func SchemaRef() string {
	return schemaRef
}

type GlobalConfig struct {
	Runner RunnerConfig `json:"runner"`
}

type RunnerConfig struct {
	Env                  Env     `json:"environment"`
	Harness              Harness `json:"harness"`
	PromptFile           string  `json:"promptFile,omitempty"`
	MaxConcurrentRunners int     `json:"maxConcurrentRunners,omitempty"` // Runners allowed on one project at once, zero means unset
}

func Load() (*GlobalConfig, error) {
	home, err := common.HomeDir()
	if err != nil {
		return nil, fmt.Errorf("could not determine home directory: %w", err)
	}

	cfgPath := common.GlobalConfigPath(home)

	exists, statErr := common.Exists(cfgPath)
	if statErr != nil {
		return DefaultConfig(), nil
	}

	var cfg GlobalConfig

	if exists {
		if err := common.ReadJSON(cfgPath, &cfg); err != nil {
			return nil, fmt.Errorf("could not parse global config: %w", err)
		}
	}

	if err := validateMaxConcurrentRunners(cfg.Runner.MaxConcurrentRunners, cfgPath); err != nil {
		return nil, err
	}

	defaultCfg := DefaultConfig()
	return mergeConfigs(defaultCfg, &cfg), nil
}

// DefaultConfig returns the built-in default global config.
func DefaultConfig() *GlobalConfig {
	return &GlobalConfig{
		Runner: RunnerConfig{
			Env:                  defaultEnv,
			Harness:              defaultHarness,
			MaxConcurrentRunners: defaultMaxConcurrentRunners,
		},
	}
}

// Fill in all missing values of the loaded config with defalt values
func mergeConfigs(defaultCfg *GlobalConfig, loadedCfg *GlobalConfig) *GlobalConfig {
	if loadedCfg.Runner.Env == "" {
		loadedCfg.Runner.Env = defaultCfg.Runner.Env
	}
	if loadedCfg.Runner.Harness == "" {
		loadedCfg.Runner.Harness = defaultCfg.Runner.Harness
	}
	if loadedCfg.Runner.MaxConcurrentRunners == 0 {
		loadedCfg.Runner.MaxConcurrentRunners = defaultCfg.Runner.MaxConcurrentRunners
	}
	return loadedCfg
}

// validateMaxConcurrentRunners rejects a negative runner limit. Zero passes, since that is what an absent key unmarshals to.
func validateMaxConcurrentRunners(value int, path string) error {
	if value < 0 {
		return fmt.Errorf("%s has %s = %d, it must be a positive number", path, maxConcurrentRunnersKey, value)
	}
	return nil
}
