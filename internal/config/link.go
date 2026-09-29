package config

import "github.com/IgorBolotnikov/DRUDGE/internal/project"

// LocalConfigLinker implements project.DirectoryLinker over the local config
// file.
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
