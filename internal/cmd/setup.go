package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"drudge/internal/common"
	"drudge/internal/config"
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
		configPath := common.GlobalConfigPath(home)
		schemaDir := filepath.Join(drudgeDir, common.SchemaDirName)
		themeSchemaPath := filepath.Join(schemaDir, common.ThemeConfigName)
		configSchemaPath := filepath.Join(schemaDir, common.GloablConfigName)
		themePath := filepath.Join(drudgeDir, common.ThemeConfigName)

		if err := common.EnsureDir(projectsDir); err != nil {
			return err
		}

		if err := common.EnsureDir(schemaDir); err != nil {
			return err
		}
		if err := os.WriteFile(themeSchemaPath, theme.Schema(), common.DefaultFilePerm); err != nil {
			return fmt.Errorf("could not write schema: %w", err)
		}
		fmt.Printf("Created %s\n", themeSchemaPath)
		if err := os.WriteFile(configSchemaPath, config.Schema(), common.DefaultFilePerm); err != nil {
			return fmt.Errorf("could not write schema: %w", err)
		}
		fmt.Printf("Created %s\n", configSchemaPath)

		cfg, err := config.Load()
		if err != nil {
			return err
		}
		globalCfg := map[string]any{
			"$schema": config.SchemaRef(),
			"drudger": map[string]any{
				"environment": cfg.Drudger.Env,
				"harness":     cfg.Drudger.Harness,
			},
		}

		wrote, err := common.WriteJSONIfNotExists(configPath, globalCfg)
		if err != nil {
			return err
		}
		if wrote {
			fmt.Printf("Created %s\n", configPath)
		} else {
			fmt.Printf("Config already exists at %s, skipping\n", configPath)
		}

		// TODO: move to config/theme.go
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
