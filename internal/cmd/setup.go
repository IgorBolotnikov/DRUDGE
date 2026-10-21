package cmd

import (
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/IgorBolotnikov/DRUDGE/internal/common"
	"github.com/IgorBolotnikov/DRUDGE/internal/setup"
)

const (
	defaultsFlagName      = "defaults"
	defaultsFlagShortName = "y"
)

var SetupCmd = &Cmd{
	Name: "setup",
	Desc: "Setup DRUDGE in this computer",
	Help: "Setup DRUDGE in this computer. In a terminal it asks for the settings of the global config. " +
		"With --defaults or input that is not a terminal it asks nothing and takes the defaults.",
	Setup: func(fs *flag.FlagSet) func(args []string) error {
		shouldTakeDefaults := fs.Bool(defaultsFlagName, false, "Skip the questions and take the default answers")
		alias(fs, defaultsFlagShortName, defaultsFlagName)
		return func([]string) error { return runSetup(*shouldTakeDefaults) }
	},
}

func runSetup(shouldTakeDefaults bool) error {
	out := newCommandPrinter()
	printProjectName(out.Theme())

	service, err := newSetupService()
	if err != nil {
		return err
	}
	answers := common.Answers{}
	if !shouldTakeDefaults && isInteractive() {
		questions, err := service.Questions()
		if err != nil {
			return err
		}
		answers, err = askQuestions(os.Stdin, os.Stdout, out.Theme(), questions)
		if errors.Is(err, errCancelled) {
			return fmt.Errorf("setup %w, nothing was written", err)
		}
		if err != nil {
			return err
		}
	}
	out.Header("Setting up DRUDGE at %s", service.DrudgeDir())
	result, err := service.Setup(answers)
	if err != nil {
		return err
	}
	for _, path := range result.SchemaPaths {
		out.Done("Created %s", path)
	}
	if result.SkillPath != "" {
		out.Done("Created %s", result.SkillPath)
	}
	for _, configFile := range []setup.ConfigFile{result.GlobalConfig, result.ThemeConfig} {
		if configFile.HasExisted {
			out.Skip("%s already exists", configFile.Path)
		} else {
			out.Done("Created %s", configFile.Path)
		}
	}
	out.Result("DRUDGE is set up, run drg project init <name> in a project directory")
	return nil
}

func newSetupService() (*setup.SetupService, error) {
	home, err := common.HomeDir()
	if err != nil {
		return nil, err
	}
	return setup.NewSetupService(home), nil
}
