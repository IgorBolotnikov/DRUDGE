package cmd

import (
	"flag"
	"fmt"

	"github.com/IgorBolotnikov/DRUDGE/internal/common"
	"github.com/IgorBolotnikov/DRUDGE/internal/setup"
)

var SetupCmd = &Cmd{
	Name:  "setup",
	Desc:  "Setup DRUDGE in this computer",
	Setup: func(*flag.FlagSet) func(args []string) error { return runSetup },
}

func runSetup([]string) error {
	printProjectName()

	service, err := newSetupService()
	if err != nil {
		return err
	}
	result, err := service.Setup()
	if err != nil {
		return err
	}

	for _, path := range result.SchemaPaths {
		fmt.Printf("Created %s\n", path)
	}
	if result.SkillPath != "" {
		fmt.Printf("Created %s\n", result.SkillPath)
	}
	if result.GlobalConfig.HasExisted {
		fmt.Printf("Config already exists at %s, skipping\n", result.GlobalConfig.Path)
	} else {
		fmt.Printf("Created %s\n", result.GlobalConfig.Path)
	}
	if result.ThemeConfig.HasExisted {
		fmt.Printf("Theme config already exists at %s, skipping\n", result.ThemeConfig.Path)
	} else {
		fmt.Printf("Created %s\n", result.ThemeConfig.Path)
	}
	fmt.Printf("Initialized DRUDGE at %s\n", result.DrudgeDir)
	return nil
}

func newSetupService() (*setup.SetupService, error) {
	home, err := common.HomeDir()
	if err != nil {
		return nil, err
	}
	return setup.NewSetupService(home), nil
}
