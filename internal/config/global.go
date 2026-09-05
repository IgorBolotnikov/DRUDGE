// Package config holds all configs which DRUDGE creates
package config

import (
	"fmt"
	"path/filepath"

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
	defaultEnv                   = EnvDockerSbx
	defaultHarness               = HarnessClaudeCode
	defaultMaxConcurrentDrudgers = 3
)

// JSON keys, named in error messages so they match what a user writes in a config file.
const (
	projectSlugKey = "projectSlug"
	promptFileKey  = "promptFile"
	// MaxConcurrentDrudgersKey is exported so the drudger package can name it when a project's pool is full.
	MaxConcurrentDrudgersKey = "maxConcurrentDrudgers"
)

// schemaRef is the $schema reference path in config.json.
const schemaRef = "./schema/config.json"

// SchemaRef returns the $schema reference path for config.json.
func SchemaRef() string {
	return schemaRef
}

type GlobalConfig struct {
	Drudger DrudgerConfig `json:"drudger"`
}

type DrudgerConfig struct {
	Env                   Env     `json:"environment"`
	Harness               Harness `json:"harness"`
	PromptFile            string  `json:"promptFile,omitempty"`
	MaxConcurrentDrudgers int     `json:"maxConcurrentDrudgers,omitempty"` // Drudgers allowed on one project at once, zero means unset
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

	if err := validatePromptFile(cfg.Drudger.PromptFile, cfgPath); err != nil {
		return nil, err
	}
	if err := validateMaxConcurrentDrudgers(cfg.Drudger.MaxConcurrentDrudgers, cfgPath); err != nil {
		return nil, err
	}

	defaultCfg := DefaultConfig()
	return mergeConfigs(defaultCfg, &cfg), nil
}

// DefaultConfig returns the built-in default global config.
func DefaultConfig() *GlobalConfig {
	return &GlobalConfig{
		Drudger: DrudgerConfig{
			Env:                   defaultEnv,
			Harness:               defaultHarness,
			MaxConcurrentDrudgers: defaultMaxConcurrentDrudgers,
		},
	}
}

// Fill in all missing values of the loaded config with defalt values
func mergeConfigs(defaultCfg *GlobalConfig, loadedCfg *GlobalConfig) *GlobalConfig {
	if loadedCfg.Drudger.Env == "" {
		loadedCfg.Drudger.Env = defaultCfg.Drudger.Env
	}
	if loadedCfg.Drudger.Harness == "" {
		loadedCfg.Drudger.Harness = defaultCfg.Drudger.Harness
	}
	if loadedCfg.Drudger.MaxConcurrentDrudgers == 0 {
		loadedCfg.Drudger.MaxConcurrentDrudgers = defaultCfg.Drudger.MaxConcurrentDrudgers
	}
	return loadedCfg
}

// validatePromptFile rejects a prompt file that is anything but a bare file
// name. An empty value passes, since that is what an absent key unmarshals to.
func validatePromptFile(value string, path string) error {
	if value == "" {
		return nil
	}
	if value == "." || value == ".." || value != filepath.Base(value) {
		return fmt.Errorf("%s has %s = %q, it must be a bare file name, prompt files are read from the prompts directory next to the config file", path, promptFileKey, value)
	}
	return nil
}

// validateMaxConcurrentDrudgers rejects a negative Drudger limit. Zero passes, since that is what an absent key unmarshals to.
func validateMaxConcurrentDrudgers(value int, path string) error {
	if value < 0 {
		return fmt.Errorf("%s has %s = %d, it must be a positive number", path, MaxConcurrentDrudgersKey, value)
	}
	return nil
}
