package config

import (
	"fmt"
	"path/filepath"
	"strings"

	"drudge/internal/common"
	"drudge/internal/task"
)

// LocalConfig scoped per project and contains overrides of global config
//
// TODO: give it a $schema pointer. It can't reuse SchemaRef, because the file
// lives outside the global dir and needs its own resolution logic
type LocalConfig struct {
	ProjectSlug           string `json:"projectSlug"`
	PromptFile            string `json:"promptFile,omitempty"`
	MaxConcurrentDrudgers int    `json:"maxConcurrentDrudgers,omitempty"`
	// DefaultTaskStatus is the status of a new task created without one. It
	// overrides the global config, and empty means unset.
	DefaultTaskStatus task.TaskStatus `json:"defaultTaskStatus,omitempty"`
	// Repositories is empty for a project initialized before drudge knew about repositories.
	Repositories []Repository `json:"repositories,omitempty"`
}

// Repository is one git repository of a project. `drg project init` writes the
// list and a user may edit it afterwards.
type Repository struct {
	// Path is where the repository sits relative to the project directory. A
	// project directory that is itself a repository records ".".
	Path string `json:"path"`
	// DefaultBranch is the branch work is cut from. An empty value means
	// drudge reads it from origin/HEAD.
	DefaultBranch string `json:"defaultBranch,omitempty"`
}

// LoadLocal reads the local config from a config file. A missing or
// slug-less file is an error.
func LoadLocal() (*LocalConfig, error) {
	path := common.LocalConfigPath()

	isPresent, err := common.Exists(path)
	if err != nil {
		return nil, err
	}
	if !isPresent {
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
	if err := validateMaxConcurrentDrudgers(cfg.MaxConcurrentDrudgers, path); err != nil {
		return nil, err
	}
	if err := validateRepositories(cfg.Repositories, path); err != nil {
		return nil, err
	}
	if err := validateDefaultTaskStatus(cfg.DefaultTaskStatus, path); err != nil {
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

// validateRepositories rejects a repository that names no path, and one whose
// path reaches outside the project directory. An empty list passes, since that
// is what an absent key unmarshals to.
func validateRepositories(repositories []Repository, path string) error {
	for _, repository := range repositories {
		if repository.Path == "" {
			return fmt.Errorf("%s has a %s entry with no %q", path, RepositoriesKey, repositoryPathKey)
		}
		if filepath.IsAbs(repository.Path) || escapesDir(repository.Path) {
			return fmt.Errorf("%s has %s entry %q, a repository path must stay inside the project directory", path, RepositoriesKey, repository.Path)
		}
	}
	return nil
}

// escapesDir tells whether a relative path climbs out of the directory it is
// relative to.
func escapesDir(value string) bool {
	cleaned := filepath.Clean(value)
	return cleaned == ".." || strings.HasPrefix(cleaned, ".."+string(filepath.Separator))
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
	if global.Drudger.PromptFile != "" {
		home, err := common.HomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(common.PromptsDir(home), global.Drudger.PromptFile), nil
	}
	return "", nil
}

// ResolveMaxConcurrentDrudgers returns how many Drudgers may work on one
// project at once, preferring the local config over the global one and
// falling back to the built-in default.
func ResolveMaxConcurrentDrudgers(local *LocalConfig, global *GlobalConfig) int {
	if local.MaxConcurrentDrudgers > 0 {
		return local.MaxConcurrentDrudgers
	}
	if global.Drudger.MaxConcurrentDrudgers > 0 {
		return global.Drudger.MaxConcurrentDrudgers
	}
	return defaultMaxConcurrentDrudgers
}

// ResolveDefaultTaskStatus returns the status of a new task created without
// one, preferring the local config over the global one and falling back to
// draft.
func ResolveDefaultTaskStatus(local *LocalConfig, global *GlobalConfig) task.TaskStatus {
	if local.DefaultTaskStatus != "" {
		return local.DefaultTaskStatus
	}
	if global.DefaultTaskStatus != "" {
		return global.DefaultTaskStatus
	}
	return task.StatusDraft
}
