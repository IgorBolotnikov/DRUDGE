// Package config holds all configs which DRUDGE creates
package config

import (
	"fmt"
	"path/filepath"
	"time"

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

// How long a sandbox command may run before DRUDGE kills it, in seconds.
// Creating a sandbox pulls an image on a first run, so it gets far more room
// than the calls that only talk to the daemon.
const (
	defaultListTimeoutSeconds   = 30
	defaultCreateTimeoutSeconds = 600
	defaultRemoveTimeoutSeconds = 120
)

// How long a git command may run before DRUDGE kills it, in seconds. Creating
// a worktree is a full checkout, so it gets far more room than the commands
// that only read refs. A fetch is capped short, so an unreachable remote costs
// seconds.
const (
	defaultFetchTimeoutSeconds      = 15
	defaultWorktreeTimeoutSeconds   = 600
	defaultGitCommandTimeoutSeconds = 60
)

// JSON keys, named in error messages so they match what a user writes in a config file.
const (
	projectSlugKey = "projectSlug"
	promptFileKey  = "promptFile"
	// MaxConcurrentDrudgersKey is exported so the drudger package can name it when a project's pool is full.
	MaxConcurrentDrudgersKey = "maxConcurrentDrudgers"
	listTimeoutKey           = "sandboxTimeouts.listSeconds"
	createTimeoutKey         = "sandboxTimeouts.createSeconds"
	removeTimeoutKey         = "sandboxTimeouts.removeSeconds"
	fetchTimeoutKey          = "gitTimeouts.fetchSeconds"
	worktreeTimeoutKey       = "gitTimeouts.worktreeSeconds"
	gitCommandTimeoutKey     = "gitTimeouts.commandSeconds"
	// RepositoriesKey and DefaultBranchKey are exported so the project package
	// can name them when a repository does not resolve.
	RepositoriesKey   = "repositories"
	DefaultBranchKey  = "defaultBranch"
	repositoryPathKey = "path"
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
	Env                   Env             `json:"environment"`
	Harness               Harness         `json:"harness"`
	PromptFile            string          `json:"promptFile,omitempty"`
	MaxConcurrentDrudgers int             `json:"maxConcurrentDrudgers,omitempty"` // Drudgers allowed on one project at once, zero means unset
	SandboxTimeouts       SandboxTimeouts `json:"sandboxTimeouts"`
	GitTimeouts           GitTimeouts     `json:"gitTimeouts"`
}

// SandboxTimeouts caps how long DRUDGE waits for each sandbox command it runs.
// A command that outruns its cap is killed. Every field is a whole number of
// seconds and zero means unset.
type SandboxTimeouts struct {
	ListSeconds   int `json:"listSeconds,omitempty"`
	CreateSeconds int `json:"createSeconds,omitempty"`
	RemoveSeconds int `json:"removeSeconds,omitempty"`
}

// List returns how long listing the sandboxes may take.
func (timeouts SandboxTimeouts) List() time.Duration {
	return time.Duration(timeouts.ListSeconds) * time.Second
}

// Create returns how long creating a sandbox may take.
func (timeouts SandboxTimeouts) Create() time.Duration {
	return time.Duration(timeouts.CreateSeconds) * time.Second
}

// Remove returns how long removing a sandbox may take.
func (timeouts SandboxTimeouts) Remove() time.Duration {
	return time.Duration(timeouts.RemoveSeconds) * time.Second
}

// GitTimeouts caps how long DRUDGE waits for each git command it runs. A
// command that outruns its cap is killed. Every field is a whole number of
// seconds and zero means unset.
type GitTimeouts struct {
	FetchSeconds    int `json:"fetchSeconds,omitempty"`
	WorktreeSeconds int `json:"worktreeSeconds,omitempty"`
	CommandSeconds  int `json:"commandSeconds,omitempty"`
}

// Fetch returns how long fetching from a remote may take.
func (timeouts GitTimeouts) Fetch() time.Duration {
	return time.Duration(timeouts.FetchSeconds) * time.Second
}

// Worktree returns how long creating a worktree may take.
func (timeouts GitTimeouts) Worktree() time.Duration {
	return time.Duration(timeouts.WorktreeSeconds) * time.Second
}

// Command returns how long every other git command may take.
func (timeouts GitTimeouts) Command() time.Duration {
	return time.Duration(timeouts.CommandSeconds) * time.Second
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
	if err := validateSandboxTimeouts(cfg.Drudger.SandboxTimeouts, cfgPath); err != nil {
		return nil, err
	}
	if err := validateGitTimeouts(cfg.Drudger.GitTimeouts, cfgPath); err != nil {
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
			SandboxTimeouts: SandboxTimeouts{
				ListSeconds:   defaultListTimeoutSeconds,
				CreateSeconds: defaultCreateTimeoutSeconds,
				RemoveSeconds: defaultRemoveTimeoutSeconds,
			},
			GitTimeouts: GitTimeouts{
				FetchSeconds:    defaultFetchTimeoutSeconds,
				WorktreeSeconds: defaultWorktreeTimeoutSeconds,
				CommandSeconds:  defaultGitCommandTimeoutSeconds,
			},
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
	if loadedCfg.Drudger.SandboxTimeouts.ListSeconds == 0 {
		loadedCfg.Drudger.SandboxTimeouts.ListSeconds = defaultCfg.Drudger.SandboxTimeouts.ListSeconds
	}
	if loadedCfg.Drudger.SandboxTimeouts.CreateSeconds == 0 {
		loadedCfg.Drudger.SandboxTimeouts.CreateSeconds = defaultCfg.Drudger.SandboxTimeouts.CreateSeconds
	}
	if loadedCfg.Drudger.SandboxTimeouts.RemoveSeconds == 0 {
		loadedCfg.Drudger.SandboxTimeouts.RemoveSeconds = defaultCfg.Drudger.SandboxTimeouts.RemoveSeconds
	}
	if loadedCfg.Drudger.GitTimeouts.FetchSeconds == 0 {
		loadedCfg.Drudger.GitTimeouts.FetchSeconds = defaultCfg.Drudger.GitTimeouts.FetchSeconds
	}
	if loadedCfg.Drudger.GitTimeouts.WorktreeSeconds == 0 {
		loadedCfg.Drudger.GitTimeouts.WorktreeSeconds = defaultCfg.Drudger.GitTimeouts.WorktreeSeconds
	}
	if loadedCfg.Drudger.GitTimeouts.CommandSeconds == 0 {
		loadedCfg.Drudger.GitTimeouts.CommandSeconds = defaultCfg.Drudger.GitTimeouts.CommandSeconds
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

// validateSandboxTimeouts rejects a negative sandbox command timeout. Zero
// passes, since that is what an absent key unmarshals to.
func validateSandboxTimeouts(timeouts SandboxTimeouts, path string) error {
	seconds := []struct {
		key   string
		value int
	}{
		{key: listTimeoutKey, value: timeouts.ListSeconds},
		{key: createTimeoutKey, value: timeouts.CreateSeconds},
		{key: removeTimeoutKey, value: timeouts.RemoveSeconds},
	}
	for _, timeout := range seconds {
		if timeout.value < 0 {
			return fmt.Errorf("%s has %s = %d, it must be a positive number of seconds", path, timeout.key, timeout.value)
		}
	}
	return nil
}

// validateGitTimeouts rejects a negative git command timeout. Zero passes,
// since that is what an absent key unmarshals to.
func validateGitTimeouts(timeouts GitTimeouts, path string) error {
	seconds := []struct {
		key   string
		value int
	}{
		{key: fetchTimeoutKey, value: timeouts.FetchSeconds},
		{key: worktreeTimeoutKey, value: timeouts.WorktreeSeconds},
		{key: gitCommandTimeoutKey, value: timeouts.CommandSeconds},
	}
	for _, timeout := range seconds {
		if timeout.value < 0 {
			return fmt.Errorf("%s has %s = %d, it must be a positive number of seconds", path, timeout.key, timeout.value)
		}
	}
	return nil
}
