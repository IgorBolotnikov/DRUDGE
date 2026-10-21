package config

import (
	"fmt"
	"slices"

	"github.com/IgorBolotnikov/DRUDGE/internal/common"
	"github.com/IgorBolotnikov/DRUDGE/internal/project"
	"github.com/IgorBolotnikov/DRUDGE/internal/remote"
)

// LocalConfigLinker implements project.DirectoryLinker and
// project.SetupRecorder over the local config file.
type LocalConfigLinker struct {
	home string
}

// NewLocalConfigLinker returns a linker whose local config files point at the
// local config schema under the drudge home directory of home.
func NewLocalConfigLinker(home string) *LocalConfigLinker {
	return &LocalConfigLinker{home: home}
}

// LinkDirectory writes a local config file holding the schema reference, the
// slug, the repositories, the Drudger env and the keys overrides sets. It
// replaces a local config file that is already there. It refuses an override
// of a key it cannot set and writes nothing then.
func (linker *LocalConfigLinker) LinkDirectory(slug string, repositories []project.Repository, envNames []string, overrides common.Answers) error {
	localCfg := LocalConfig{
		Schema:       LocalSchemaRef(linker.home),
		ProjectSlug:  slug,
		Drudger:      LocalDrudgerConfig{Env: envNames},
		Repositories: repositories,
	}
	for _, name := range envNames {
		if !slices.Contains(localCfg.Drudger.Env, name) {
			localCfg.Drudger.Env = append(localCfg.Drudger.Env, name)
		}
	}
	for key, answer := range overrides {
		if err := localCfg.setAnswer(key, answer); err != nil {
			return err
		}
	}
	return localCfg.Save()
}

// setAnswer sets key of the local config to the value of answer.
func (cfg *LocalConfig) setAnswer(key string, answer string) error {
	switch key {
	case maxConcurrentDrudgersKey:
		value, err := common.ParseNumberAnswer(answer)
		if err != nil {
			return fmt.Errorf("answer for %s: %w", key, err)
		}
		cfg.MaxConcurrentDrudgers = value
	case branchFormatKey:
		cfg.Task.BranchFormat = answer
	case remoteProviderKey:
		cfg.Remote.Provider = remote.Provider(answer)
	case PullRequestsEnabledKey:
		cfg.Remote.PullRequests.IsEnabled = new(answer == common.AnswerYes)
	case PullRequestsDraftKey:
		cfg.Remote.PullRequests.IsDraft = new(answer == common.AnswerYes)
	default:
		return fmt.Errorf("the local config cannot take an answer for %s", key)
	}
	return nil
}

// LinkedSlug reads the project slug out of the local config file. A directory
// with no local config file is linked to no project, and an invalid one is an
// error.
func (linker *LocalConfigLinker) LinkedSlug() (slug string, isLinked bool, err error) {
	isPresent, err := common.Exists(common.LocalConfigPath())
	if err != nil || !isPresent {
		return "", false, err
	}
	localCfg, err := LoadLocal()
	if err != nil {
		return "", false, err
	}
	return localCfg.ProjectSlug, true, nil
}

// RecordedSetup reads the repositories and the Drudger env names out of the
// local config file. A directory with no local config file is an error.
func (linker *LocalConfigLinker) RecordedSetup() (repositories []project.Repository, envNames []string, err error) {
	localCfg, err := LoadLocal()
	if err != nil {
		return nil, nil, err
	}
	return localCfg.Repositories, localCfg.Drudger.Env, nil
}

// RecordSetup sets the setup and the copy paths of every repository of the
// local config that setups names, and leaves the other repositories and keys
// as they are. A nil copy keeps the copy paths of the repository. Every name
// of envNames missing from the Drudger env is appended to it.
func (linker *LocalConfigLinker) RecordSetup(setups map[string]project.DetectedSetup, envNames []string) error {
	localCfg, err := LoadLocal()
	if err != nil {
		return err
	}
	for index, repository := range localCfg.Repositories {
		detected, ok := setups[repository.Path]
		if !ok {
			continue
		}
		localCfg.Repositories[index].Setup = detected.Setup
		if detected.Copy != nil {
			localCfg.Repositories[index].Copy = detected.Copy
		}
	}
	for _, name := range envNames {
		if !slices.Contains(localCfg.Drudger.Env, name) {
			localCfg.Drudger.Env = append(localCfg.Drudger.Env, name)
		}
	}
	return localCfg.Save()
}
