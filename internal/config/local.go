package config

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/IgorBolotnikov/DRUDGE/internal/common"
	"github.com/IgorBolotnikov/DRUDGE/internal/project"
	"github.com/IgorBolotnikov/DRUDGE/internal/task"
)

// LocalConfig scoped per project and contains overrides of global config
type LocalConfig struct {
	// Schema is empty for a project initialized before the local config had a schema.
	Schema                string             `json:"$schema,omitempty"`
	ProjectSlug           string             `json:"projectSlug"`
	PromptFile            string             `json:"promptFile,omitempty"`
	MaxConcurrentDrudgers int                `json:"maxConcurrentDrudgers,omitempty"`
	Task                  TaskConfig         `json:"task,omitzero"`
	Drudger               LocalDrudgerConfig `json:"drudger,omitzero"`
	Remote                RemoteConfig       `json:"remote,omitzero"`
	// Repositories is empty for a project initialized before drudge knew about repositories.
	Repositories []project.Repository `json:"repositories,omitempty"`
}

// LocalDrudgerConfig holds the Drudger settings a local config overrides.
type LocalDrudgerConfig struct {
	PageSize *int `json:"pageSize,omitempty"` // Drudgers on one page of a listing, nil means unset and zero means every Drudger on one page
}

// LocalSchemaRef returns the $schema reference for the local config file, the
// absolute path of the local config schema. The local config file lives outside
// the drudge home directory, so a relative reference would not resolve.
func LocalSchemaRef(home string) string {
	return filepath.Join(common.DrudgeDir(home), common.SchemaDirName, common.LocalSchemaName)
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
	if err := validatePromptFile(cfg.PromptFile, promptFileKey, path); err != nil {
		return nil, err
	}
	if err := validateMaxConcurrentDrudgers(cfg.MaxConcurrentDrudgers, path); err != nil {
		return nil, err
	}
	if err := validateRepositories(cfg.Repositories, path); err != nil {
		return nil, err
	}
	if err := validateDefaultTaskStatus(cfg.Task.DefaultStatus, path); err != nil {
		return nil, err
	}
	if err := validatePageSize(cfg.Task.PageSize, TaskPageSizeKey, "task", path); err != nil {
		return nil, err
	}
	if err := validatePageSize(cfg.Drudger.PageSize, DrudgerPageSizeKey, "Drudger", path); err != nil {
		return nil, err
	}
	if err := validateRemote(cfg.Remote, path); err != nil {
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
func validateRepositories(repositories []project.Repository, path string) error {
	for _, repository := range repositories {
		if repository.Path == "" {
			return fmt.Errorf("%s has a %s entry with no %q", path, project.RepositoriesKey, project.RepositoryPathKey)
		}
		if filepath.IsAbs(repository.Path) || escapesDir(repository.Path) {
			return fmt.Errorf("%s has %s entry %q, a repository path must stay inside the project directory", path, project.RepositoriesKey, repository.Path)
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

// ResolvePromptPath returns the path of the prompt file to hand an agent. An
// empty path means neither config names a prompt file and the built-in default
// applies.
func ResolvePromptPath(local *LocalConfig, global *GlobalConfig) (string, error) {
	return resolvePromptsFile(local.PromptFile, global.Drudger.PromptFile)
}

// resolvePromptsFile returns the path of a file of the prompts directory,
// preferring the file the local config names over the one the global config
// names. A local file lives in the prompts directory of the local drudge dir, a
// global one in the prompts directory of the drudge home directory. An empty
// path means neither config names one.
func resolvePromptsFile(localName string, globalName string) (string, error) {
	if localName != "" {
		return filepath.Join(common.LocalPromptsDir(), localName), nil
	}
	if globalName != "" {
		home, err := common.HomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(common.PromptsDir(home), globalName), nil
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
	if local.Task.DefaultStatus != "" {
		return local.Task.DefaultStatus
	}
	if global.Task.DefaultStatus != "" {
		return global.Task.DefaultStatus
	}
	return task.StatusDraft
}

// ResolveTaskPageSize returns how many tasks one page of a listing holds,
// preferring the local config over the global one and falling back to the
// default page size. A local size of 0 wins over a global size.
func ResolveTaskPageSize(local *LocalConfig, global *GlobalConfig) int {
	if local.Task.PageSize != nil {
		return *local.Task.PageSize
	}
	if global.Task.PageSize != nil {
		return *global.Task.PageSize
	}
	return defaultTaskPageSize
}

// ResolveDrudgerPageSize returns how many Drudgers one page of a listing
// holds, preferring the local config over the global one and falling back to
// the default page size. A local size of 0 wins over a global size.
func ResolveDrudgerPageSize(local *LocalConfig, global *GlobalConfig) int {
	if local.Drudger.PageSize != nil {
		return *local.Drudger.PageSize
	}
	if global.Drudger.PageSize != nil {
		return *global.Drudger.PageSize
	}
	return defaultDrudgerPageSize
}
