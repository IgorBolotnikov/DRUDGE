package config

import (
	"fmt"
	"slices"
	"time"

	"github.com/IgorBolotnikov/DRUDGE/internal/remote"
)

// defaultRemoteTimeoutSeconds caps one call of the provider CLI.
const defaultRemoteTimeoutSeconds = 60

const (
	remoteProviderKey = "remote.provider"
	remoteTimeoutKey  = remote.TimeoutKey
	pullRequestsKey   = remote.PullRequestsEnabledKey
	templateFileKey   = "remote.pullRequests.templateFile"
	stepsFileKey      = "remote.pullRequests.stepsFile"
)

// defaultTitleFormat is what the agent is told to write as the title of a
// pull request.
const defaultTitleFormat = "<a short summary of the change>"

// RemoteConfig holds the settings for the provider the repositories of a
// project are hosted on. A local config overrides each field it sets.
type RemoteConfig struct {
	Provider       remote.Provider    `json:"provider,omitempty"`       // Empty means unset
	TimeoutSeconds int                `json:"timeoutSeconds,omitempty"` // Zero means unset
	PullRequests   PullRequestsConfig `json:"pullRequests,omitzero"`
}

// PullRequestsConfig holds the settings for the pull requests DRUDGE opens.
// Every field is nil when unset.
type PullRequestsConfig struct {
	IsEnabled    *bool  `json:"isEnabled,omitempty"`
	IsDraft      *bool  `json:"isDraft,omitempty"`
	TitleFormat  string `json:"titleFormat,omitempty"`
	TemplateFile string `json:"templateFile,omitempty"`
	StepsFile    string `json:"stepsFile,omitempty"`
}

// RemoteSettings is the remote config with the local config, the global
// config and the defaults merged.
type RemoteSettings struct {
	Provider     remote.Provider
	Timeout      time.Duration
	PullRequests PullRequestSettings
}

// PullRequestSettings is the pull request config with the local config, the
// global config and the defaults merged.
type PullRequestSettings struct {
	IsEnabled   bool
	IsDraft     bool
	TitleFormat string
	// TemplatePath and StepsPath are empty when neither config names the file.
	TemplatePath string
	StepsPath    string
}

// ResolveRemote merges the remote settings of the local config over the
// global one and the defaults. It refuses pull requests turned on with no
// provider set in either config.
func ResolveRemote(local *LocalConfig, global *GlobalConfig) (RemoteSettings, error) {
	localPullRequests := local.Remote.PullRequests
	globalPullRequests := global.Remote.PullRequests

	templatePath, err := resolvePromptsFile(localPullRequests.TemplateFile, globalPullRequests.TemplateFile)
	if err != nil {
		return RemoteSettings{}, err
	}
	stepsPath, err := resolvePromptsFile(localPullRequests.StepsFile, globalPullRequests.StepsFile)
	if err != nil {
		return RemoteSettings{}, err
	}

	settings := RemoteSettings{
		Provider: firstSet(local.Remote.Provider, global.Remote.Provider),
		Timeout:  time.Duration(firstSet(local.Remote.TimeoutSeconds, global.Remote.TimeoutSeconds, defaultRemoteTimeoutSeconds)) * time.Second,
		PullRequests: PullRequestSettings{
			IsEnabled:    *firstSet(localPullRequests.IsEnabled, globalPullRequests.IsEnabled, new(false)),
			IsDraft:      *firstSet(localPullRequests.IsDraft, globalPullRequests.IsDraft, new(false)),
			TitleFormat:  firstSet(localPullRequests.TitleFormat, globalPullRequests.TitleFormat, defaultTitleFormat),
			TemplatePath: templatePath,
			StepsPath:    stepsPath,
		},
	}
	if settings.PullRequests.IsEnabled && settings.Provider == "" {
		return RemoteSettings{}, fmt.Errorf(
			"%s is true and %s is not set, set it to one of %v in the global or the local config, or turn off %s",
			pullRequestsKey, remoteProviderKey, remote.Providers, pullRequestsKey,
		)
	}
	return settings, nil
}

// firstSet returns the first value that is not the zero value of its type.
func firstSet[T comparable](values ...T) T {
	var zero T
	for _, value := range values {
		if value != zero {
			return value
		}
	}
	return zero
}

// validateRemote rejects a provider DRUDGE does not support, a negative
// timeout and a template or steps file that is not a bare file name. Unset
// fields pass.
func validateRemote(cfg RemoteConfig, path string) error {
	if cfg.Provider != "" && !slices.Contains(remote.Providers, cfg.Provider) {
		return fmt.Errorf("%s has %s = %q, it must be one of %v", path, remoteProviderKey, cfg.Provider, remote.Providers)
	}
	if cfg.TimeoutSeconds < 0 {
		return fmt.Errorf("%s has %s = %d, it must be a positive number of seconds", path, remoteTimeoutKey, cfg.TimeoutSeconds)
	}
	if err := validatePromptFile(cfg.PullRequests.TemplateFile, templateFileKey, path); err != nil {
		return err
	}
	return validatePromptFile(cfg.PullRequests.StepsFile, stepsFileKey, path)
}
