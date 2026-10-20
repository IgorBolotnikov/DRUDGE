package config

import (
	"github.com/IgorBolotnikov/DRUDGE/internal/common"
	"github.com/IgorBolotnikov/DRUDGE/internal/project"
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
// slug and the repositories. It replaces a local config file that is already
// there.
func (linker *LocalConfigLinker) LinkDirectory(slug string, repositories []project.Repository) error {
	localCfg := LocalConfig{
		Schema:       LocalSchemaRef(linker.home),
		ProjectSlug:  slug,
		Repositories: repositories,
	}
	return localCfg.Save()
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

// RecordSetup sets the setup and the copy paths of every repository of the
// local config that setups names, and leaves the other repositories and keys
// as they are. A nil copy keeps the copy paths of the repository.
func (linker *LocalConfigLinker) RecordSetup(setups map[string]project.DetectedSetup) error {
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
	return localCfg.Save()
}
