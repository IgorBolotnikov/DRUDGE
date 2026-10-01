package cmd

import (
	"flag"
	"fmt"
)

var CleanupCmd = &Cmd{
	Name: "cleanup",
	Desc: "Cleanup DRUDGE from this computer",
	Setup: func(fs *flag.FlagSet) func(args []string) error {
		isForced := fs.Bool(forceFlagName, false, "Remove without asking")
		alias(fs, forceFlagShortName, forceFlagName)
		return func([]string) error { return cleanup(*isForced) }
	},
}

func cleanup(isForced bool) error {
	service, err := newSetupService()
	if err != nil {
		return err
	}

	if !isForced {
		isInstalled, err := service.IsInstalled()
		if err != nil {
			return err
		}
		if !isInstalled {
			printNothingToCleanUp(service.DrudgeDir())
			return nil
		}
		isConfirmed, err := ConfirmDeletion(newCommandPrinter(), service.DrudgeDir())
		if err != nil {
			return err
		}
		if !isConfirmed {
			fmt.Println("Aborted")
			return nil
		}
	}

	result, err := service.Cleanup()
	if err != nil {
		return err
	}
	if !result.HasRemoved {
		printNothingToCleanUp(result.DrudgeDir)
		return nil
	}
	fmt.Printf("Removed %s\n", result.DrudgeDir)
	return nil
}

func printNothingToCleanUp(drudgeDir string) {
	fmt.Printf("Nothing to clean up, %s does not exist\n", drudgeDir)
}
