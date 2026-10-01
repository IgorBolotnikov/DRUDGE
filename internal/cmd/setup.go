package cmd

import (
	"flag"

	"github.com/IgorBolotnikov/DRUDGE/internal/common"
	"github.com/IgorBolotnikov/DRUDGE/internal/setup"
	"github.com/IgorBolotnikov/DRUDGE/internal/theme"
)

var SetupCmd = &Cmd{
	Name:  "setup",
	Desc:  "Setup DRUDGE in this computer",
	Setup: func(*flag.FlagSet) func(args []string) error { return runSetup },
}

func runSetup([]string) error {
	palette := theme.LoadOrDefault()
	printProjectName(palette)

	service, err := newSetupService()
	if err != nil {
		return err
	}
	out := newPrinter(newThemedLogger(palette), palette)
	out.header("Setting up DRUDGE at %s", service.DrudgeDir())
	result, err := service.Setup()
	if err != nil {
		return err
	}
	for _, path := range result.SchemaPaths {
		out.done("Created %s", path)
	}
	if result.SkillPath != "" {
		out.done("Created %s", result.SkillPath)
	}
	for _, configFile := range []setup.ConfigFile{result.GlobalConfig, result.ThemeConfig} {
		if configFile.HasExisted {
			out.skip("%s already exists", configFile.Path)
		} else {
			out.done("Created %s", configFile.Path)
		}
	}
	out.result("DRUDGE is set up, run drg project init <name> in a project directory")
	return nil
}

func newSetupService() (*setup.SetupService, error) {
	home, err := common.HomeDir()
	if err != nil {
		return nil, err
	}
	return setup.NewSetupService(home), nil
}
