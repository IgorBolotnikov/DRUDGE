package config

import (
	"fmt"
	"path/filepath"

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
	if err := validatePromptFile(cfg.PromptFile, path); err != nil {
		return nil, err
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

// ResolvePromptPath returns the path of the prompt file to hand an agent,
// preferring the local config over the global one. A local prompt file lives
// in the prompts directory of the local drudge dir, a global one in the
// prompts directory of the drudge home directory. An empty path means neither
// config names a prompt file and the built-in default applies.
func ResolvePromptPath(local *LocalConfig, global *GlobalConfig) (string, error) {
	if local.PromptFile != "" {
		return filepath.Join(common.LocalPromptsDir(), local.PromptFile), nil
	}
	if global.Runner.PromptFile != "" {
		home, err := common.HomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(common.PromptsDir(home), global.Runner.PromptFile), nil
	}
	return "", nil
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
