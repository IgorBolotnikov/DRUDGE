// Package config holds all configs which DRUDGE creates
package config

import (
	"fmt"
	"path/filepath"
	"slices"
	"time"

	"github.com/IgorBolotnikov/DRUDGE/internal/common"
	"github.com/IgorBolotnikov/DRUDGE/internal/drudger"
	"github.com/IgorBolotnikov/DRUDGE/internal/task"
)

// Env and Harness are the drudger types, named here for the config files.
type Env = drudger.Env

const (
	EnvDockerSbx = drudger.EnvDockerSbx
)

type Harness = drudger.Harness

const (
	HarnessClaudeCode = drudger.HarnessClaudeCode
	HarnessOpencode   = drudger.HarnessOpencode
)

const (
	defaultEnv                   = EnvDockerSbx
	defaultHarness               = HarnessClaudeCode
	defaultMaxConcurrentDrudgers = 3
	defaultProjectPageSize       = 20
	defaultTaskPageSize          = 20
	defaultDrudgerPageSize       = 20
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
	projectSlugKey           = "projectSlug"
	promptFileKey            = "promptFile"
	maxConcurrentDrudgersKey = drudger.MaxConcurrentDrudgersKey
	listTimeoutKey           = "sandboxTimeouts.listSeconds"
	createTimeoutKey         = drudger.CreateTimeoutKey
	removeTimeoutKey         = "sandboxTimeouts.removeSeconds"
	fetchTimeoutKey          = "gitTimeouts.fetchSeconds"
	worktreeTimeoutKey       = "gitTimeouts.worktreeSeconds"
	gitCommandTimeoutKey     = "gitTimeouts.commandSeconds"
	branchFormatKey          = "task.branchFormat"
	// DefaultTaskStatusKey is exported so the task commands can name it in their help.
	DefaultTaskStatusKey = "task.defaultStatus"
	// ProjectPageSizeKey is exported so the project commands can name it in their help.
	ProjectPageSizeKey = "project.pageSize"
	// TaskPageSizeKey is exported so the task commands can name it in their help.
	TaskPageSizeKey = "task.pageSize"
	// DrudgerPageSizeKey is exported so the drudger commands can name it in their help.
	DrudgerPageSizeKey = "drudger.pageSize"
)

// defaultTaskStatuses are the statuses a config may give a new task.
var defaultTaskStatuses = []task.TaskStatus{task.StatusDraft, task.StatusTodo}

// schemaRef is the $schema reference path in config.json.
const schemaRef = "./schema/config.json"

// SchemaRef returns the $schema reference path for config.json.
func SchemaRef() string {
	return schemaRef
}

// DefaultDocument is the global config file drg setup writes when there is none.
type DefaultDocument struct {
	Schema  string                 `json:"$schema"`
	Drudger DefaultDrudgerDocument `json:"drudger"`
}

// DefaultDrudgerDocument is the drudger section of DefaultDocument.
type DefaultDrudgerDocument struct {
	Env     Env     `json:"environment"`
	Harness Harness `json:"harness"`
}

// NewDefaultDocument returns the default global config file.
func NewDefaultDocument() DefaultDocument {
	return DefaultDocument{
		Schema:  schemaRef,
		Drudger: DefaultDrudgerDocument{Env: defaultEnv, Harness: defaultHarness},
	}
}

type GlobalConfig struct {
	Drudger DrudgerConfig `json:"drudger"`
	Task    TaskConfig    `json:"task,omitzero"`
	Project ProjectConfig `json:"project,omitzero"`
	Remote  RemoteConfig  `json:"remote,omitzero"`
}

// ProjectConfig holds the settings for listing projects.
type ProjectConfig struct {
	PageSize *int `json:"pageSize,omitempty"` // Projects on one page of a listing, nil means unset and zero means every project on one page
}

// TaskConfig holds the settings for the tasks of a project. A local config
// overrides each field it sets.
type TaskConfig struct {
	DefaultStatus task.TaskStatus `json:"defaultStatus,omitempty"` // Status of a new task created without one, empty means unset
	PageSize      *int            `json:"pageSize,omitempty"`      // Tasks on one page of a listing, nil means unset and zero means every task on one page
	BranchFormat  string          `json:"branchFormat,omitempty"`  // Format of the branch names of tasks, empty means unset
}

type DrudgerConfig struct {
	Env                   Env             `json:"environment"`
	Harness               Harness         `json:"harness"`
	PromptFile            string          `json:"promptFile,omitempty"`
	MaxConcurrentDrudgers int             `json:"maxConcurrentDrudgers,omitempty"` // Drudgers allowed on one project at once, zero means unset
	SandboxTimeouts       SandboxTimeouts `json:"sandboxTimeouts"`
	GitTimeouts           GitTimeouts     `json:"gitTimeouts"`
	PageSize              *int            `json:"pageSize,omitempty"` // Drudgers on one page of a listing, nil means unset and zero means every Drudger on one page
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

	isPresent, statErr := common.Exists(cfgPath)
	if statErr != nil {
		return DefaultConfig(), nil
	}

	var cfg GlobalConfig

	if isPresent {
		if err := common.ReadJSON(cfgPath, &cfg); err != nil {
			return nil, fmt.Errorf("could not parse global config: %w", err)
		}
	}

	if err := validateDefaultTaskStatus(cfg.Task.DefaultStatus, cfgPath); err != nil {
		return nil, err
	}
	if err := validateBranchFormat(cfg.Task.BranchFormat, cfgPath); err != nil {
		return nil, err
	}
	if err := validatePromptFile(cfg.Drudger.PromptFile, promptFileKey, cfgPath); err != nil {
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
	if err := validatePageSize(cfg.Project.PageSize, ProjectPageSizeKey, "project", cfgPath); err != nil {
		return nil, err
	}
	if err := validatePageSize(cfg.Task.PageSize, TaskPageSizeKey, "task", cfgPath); err != nil {
		return nil, err
	}
	if err := validatePageSize(cfg.Drudger.PageSize, DrudgerPageSizeKey, "Drudger", cfgPath); err != nil {
		return nil, err
	}
	if err := validateRemote(cfg.Remote, cfgPath); err != nil {
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

// ResolveProjectPageSize returns how many projects one page of a listing
// holds, falling back to the default page size.
func ResolveProjectPageSize(global *GlobalConfig) int {
	if global.Project.PageSize != nil {
		return *global.Project.PageSize
	}
	return defaultProjectPageSize
}

// validatePromptFile rejects a file of the prompts directory that is anything
// but a bare file name. An empty value passes, since that is what an absent key
// unmarshals to.
func validatePromptFile(value string, key string, path string) error {
	if value == "" {
		return nil
	}
	if value == "." || value == ".." || value != filepath.Base(value) {
		return fmt.Errorf("%s has %s = %q, it must be a bare file name, prompt files are read from the prompts directory next to the config file", path, key, value)
	}
	return nil
}

// validateDefaultTaskStatus rejects a default task status other than draft
// and todo. An empty value passes, since that is what an absent key
// unmarshals to.
func validateDefaultTaskStatus(value task.TaskStatus, path string) error {
	if value == "" || slices.Contains(defaultTaskStatuses, value) {
		return nil
	}
	return fmt.Errorf("%s has %s = %q, it must be one of %s", path, DefaultTaskStatusKey, value, task.FormatStatuses(defaultTaskStatuses))
}

// validateBranchFormat rejects a branch name format the task package refuses.
// An empty value passes, since that is what an absent key unmarshals to.
func validateBranchFormat(value string, path string) error {
	if value == "" {
		return nil
	}
	if err := task.ValidateBranchFormat(value); err != nil {
		return fmt.Errorf("%s has %s = %q, %w", path, branchFormatKey, value, err)
	}
	return nil
}

// validateMaxConcurrentDrudgers rejects a negative Drudger limit. Zero passes, since that is what an absent key unmarshals to.
func validateMaxConcurrentDrudgers(value int, path string) error {
	if value < 0 {
		return fmt.Errorf("%s has %s = %d, it must be a positive number", path, maxConcurrentDrudgersKey, value)
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

// validatePageSize rejects a negative page size. A nil value passes, since
// that is what an absent key unmarshals to. noun names what a page holds in
// the message.
func validatePageSize(value *int, key string, noun string, path string) error {
	if value != nil && *value < 0 {
		return fmt.Errorf("%s has %s = %d, it must be 0 or more, 0 puts every %s on one page", path, key, *value, noun)
	}
	return nil
}
