package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"drudge/internal/common"
	"drudge/internal/theme"
)

var SetupCmd = &Cmd{
	Name:  "setup",
	Usage: "setup",
	Desc:  "Setup DRUDGE in this computer",
	Run: func(args []string) error {
		home, err := common.HomeDir()
		if err != nil {
			return err
		}

		drudgeDir := common.DrudgeDir(home)
		projectsDir := common.ProjectsDir(home)
		configPath := filepath.Join(drudgeDir, common.DrudgeConfigName)
		schemaDir := filepath.Join(drudgeDir, common.SchemaDirName)
		schemaPath := filepath.Join(schemaDir, theme.ThemeConfigName)
		themePath := filepath.Join(drudgeDir, theme.ThemeConfigName)

		if err := common.EnsureDir(projectsDir); err != nil {
			return err
		}

		defaultConfig := map[string]any{}

		wrote, err := common.WriteJSONIfNotExists(configPath, defaultConfig)
		if err != nil {
			return err
		}
		if wrote {
			fmt.Printf("Created %s\n", configPath)
		} else {
			fmt.Printf("Config already exists at %s, skipping\n", configPath)
		}

		if err := common.EnsureDir(schemaDir); err != nil {
			return err
		}
		if err := os.WriteFile(schemaPath, theme.Schema(), common.DefaultFilePerm); err != nil {
			return fmt.Errorf("could not write schema: %w", err)
		}
		fmt.Printf("Created %s\n", schemaPath)

		themeCfg := map[string]any{
			"$schema":   theme.ThemeSchemaRef(),
			"theme":     theme.DefaultTheme(),
			"overrides": map[string]any{},
		}
		wrote, err = common.WriteJSONIfNotExists(themePath, themeCfg)
		if err != nil {
			return err
		}
		if wrote {
			fmt.Printf("Created %s\n", themePath)
		} else {
			fmt.Printf("Theme config already exists at %s, skipping\n", themePath)
		}

		fmt.Printf("Initialized DRUDGE at %s\n", drudgeDir)
		return nil
	},
}
