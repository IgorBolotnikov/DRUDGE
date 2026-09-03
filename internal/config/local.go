package config

import (
	"fmt"

	"drudge/internal/common"
)

// LocalConfig scoped per project and contains overrides of global config
//
// TODO: give it a $schema pointer. It can't reuse SchemaRef, because the file
// lives outside the global dir and needs its own resolution logic
type LocalConfig struct {
	ProjectSlug          string `json:"projectSlug"`
	PromptFile           string `json:"promptFile,omitempty"`
	MaxConcurrentRunners int    `json:"maxConcurrentRunners,omitempty"`
}

// LoadLocal reads the local config from a config file. A missing or
// slug-less file is an error.
func LoadLocal() (*LocalConfig, error) {
	path := common.LocalConfigPath()

	exists, err := common.Exists(path)
	if err != nil {
		return nil, err
	}
	if !exists {
		// TODO: don't hardcode the commands inject them in the string template
		// otherwise it'll bite us in the arse when we want to change the commands
		return nil, fmt.Errorf("no project initialized in current directory (%s not found), run `drg project init <name>` first", path)
	}

	var cfg LocalConfig
	if err := common.ReadJSON(path, &cfg); err != nil {
		return nil, err
	}

	if cfg.ProjectSlug == "" {
		return nil, fmt.Errorf("%s is missing %q", path, projectSlugKey)
	}
	if err := validateMaxConcurrentRunners(cfg.MaxConcurrentRunners, path); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// Save writes the config to local config file, creating the config dir
// directory if needed.
func (cfg *LocalConfig) Save() error {
	if err := common.EnsureDir(common.DotDrudgeDirName); err != nil {
		return err
	}
	return common.WriteJSON(common.LocalConfigPath(), cfg)
}

// ResolvePromptFile returns the prompt file name to hand an agent, preferring
// the local config over the global one and falling back to the build-in
// default.
func ResolvePromptFile(local *LocalConfig, global *GlobalConfig) string {
	if local.PromptFile != "" {
		return local.PromptFile
	}
	if global.Runner.PromptFile != "" {
		return global.Runner.PromptFile
	}
	return ""
}

// ResolveMaxConcurrentRunners returns how many runners may work on one
// project at once, preferring the local config over the global one and
// falling back to the built-in default.
func ResolveMaxConcurrentRunners(local *LocalConfig, global *GlobalConfig) int {
	if local.MaxConcurrentRunners > 0 {
		return local.MaxConcurrentRunners
	}
	if global.Runner.MaxConcurrentRunners > 0 {
		return global.Runner.MaxConcurrentRunners
	}
	return defaultMaxConcurrentRunners
}
