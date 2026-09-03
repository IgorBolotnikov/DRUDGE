package config

import (
	"fmt"

	"drudge/internal/common"
)

// LocalConfig links the current directory to a project and overrides runner
// settings for that project only. Written by `drg project init`.
//
// TODO: give it a $schema pointer. It can't reuse SchemaRef, because the file
// lives outside ~/.drudge and needs its own resolution story.
type LocalConfig struct {
	ProjectSlug          string `json:"projectSlug"`
	PromptFile           string `json:"promptFile,omitempty"`           // File name under ./.drudge/prompts/
	MaxConcurrentRunners int    `json:"maxConcurrentRunners,omitempty"` // Runners allowed on this project at once, zero means unset
}

// LoadLocal reads the local config from ./.drudge/config.json. A missing or
// slug-less file is an error, since task commands have no project to work on
// without it.
func LoadLocal() (*LocalConfig, error) {
	path := common.LocalConfigPath()

	exists, err := common.Exists(path)
	if err != nil {
		return nil, err
	}
	if !exists {
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

// Save writes the config to ./.drudge/config.json, creating the .drudge
// directory if needed.
func (cfg *LocalConfig) Save() error {
	if err := common.EnsureDir(common.DotDrudgeDirName); err != nil {
		return err
	}
	return common.WriteJSON(common.LocalConfigPath(), cfg)
}

// ResolvePromptFile returns the prompt file name to hand an agent, preferring
// the local config over the global one. An empty result means the built-in
// default prompt applies.
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
