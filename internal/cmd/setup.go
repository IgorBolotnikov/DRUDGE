package cmd

import (
	"fmt"
	"path/filepath"

	"drudge/internal/common"
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
		configPath := filepath.Join(drudgeDir, "config.json")

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

		fmt.Printf("Initialized DRUDGE at %s\n", drudgeDir)
		return nil
	},
}
