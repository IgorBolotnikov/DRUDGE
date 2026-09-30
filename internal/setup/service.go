package setup

import (
	"path/filepath"

	"github.com/IgorBolotnikov/DRUDGE/internal/common"
	"github.com/IgorBolotnikov/DRUDGE/internal/config"
	"github.com/IgorBolotnikov/DRUDGE/internal/skill"
	"github.com/IgorBolotnikov/DRUDGE/internal/theme"
)

type SetupService struct {
	home string
}

func NewSetupService(home string) *SetupService {
	return &SetupService{home: home}
}

// DrudgeDir returns the drudge home directory Setup creates and Cleanup
// removes.
func (s *SetupService) DrudgeDir() string {
	return common.DrudgeDir(s.home)
}

// Setup creates the drudge home directory, writes the schema files and
// installs the skill for the configured harness. The schemas and the skill
// are overwritten on every run. The global config and the theme config are
// written only when they are missing.
func (s *SetupService) Setup() (*SetupResult, error) {
	drudgeDir := s.DrudgeDir()
	result := &SetupResult{DrudgeDir: drudgeDir}

	if err := common.EnsureDir(common.ProjectsDir(s.home)); err != nil {
		return nil, err
	}

	schemaDir := filepath.Join(drudgeDir, common.SchemaDirName)
	if err := common.EnsureDir(schemaDir); err != nil {
		return nil, err
	}
	schemas := []struct {
		name    string
		content []byte
	}{
		{common.ThemeConfigName, theme.Schema()},
		{common.GloablConfigName, config.Schema()},
		{common.LocalSchemaName, config.LocalSchema()},
	}
	for _, schema := range schemas {
		path := filepath.Join(schemaDir, schema.name)
		if err := common.WriteFile(path, string(schema.content)); err != nil {
			return nil, err
		}
		result.SchemaPaths = append(result.SchemaPaths, path)
	}

	globalConfig, err := config.Load()
	if err != nil {
		return nil, err
	}
	skillPath, didInstall, err := skill.InstallDrudge(s.home, globalConfig.Drudger.Harness)
	if err != nil {
		return nil, err
	}
	if didInstall {
		result.SkillPath = skillPath
	}

	result.GlobalConfig, err = writeConfigFile(common.GlobalConfigPath(s.home), config.NewDefaultDocument())
	if err != nil {
		return nil, err
	}
	result.ThemeConfig, err = writeConfigFile(common.ThemeConfigPath(s.home), theme.NewDefaultDocument())
	if err != nil {
		return nil, err
	}
	return result, nil
}

// IsInstalled reports whether the drudge home directory exists.
func (s *SetupService) IsInstalled() (bool, error) {
	return common.Exists(s.DrudgeDir())
}

// Cleanup removes the drudge home directory and everything in it.
func (s *SetupService) Cleanup() (*CleanupResult, error) {
	drudgeDir := s.DrudgeDir()
	isInstalled, err := s.IsInstalled()
	if err != nil {
		return nil, err
	}
	if !isInstalled {
		return &CleanupResult{DrudgeDir: drudgeDir}, nil
	}
	if err := common.RemoveAll(drudgeDir); err != nil {
		return nil, err
	}
	return &CleanupResult{DrudgeDir: drudgeDir, HasRemoved: true}, nil
}

func writeConfigFile(path string, document any) (ConfigFile, error) {
	didWrite, err := common.WriteJSONIfNotExists(path, document)
	if err != nil {
		return ConfigFile{}, err
	}
	return ConfigFile{Path: path, HasExisted: !didWrite}, nil
}
