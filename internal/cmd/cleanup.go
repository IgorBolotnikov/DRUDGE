package cmd

import (
	"flag"
	"fmt"

	"github.com/IgorBolotnikov/DRUDGE/internal/common"
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
	home, err := common.HomeDir()
	if err != nil {
		return err
	}

	drudgeDir := common.DrudgeDir(home)

	isPresent, err := common.Exists(drudgeDir)
	if err != nil {
		return err
	}
	if !isPresent {
		fmt.Printf("Nothing to clean up, %s does not exist\n", drudgeDir)
		return nil
	}

	if !isForced {
		isConfirmed, err := ConfirmDeletion(drudgeDir)
		if err != nil {
			return err
		}
		if !isConfirmed {
			fmt.Println("Aborted")
			return nil
		}
	}

	if err := common.RemoveAll(drudgeDir); err != nil {
		return err
	}

	fmt.Printf("Removed %s\n", drudgeDir)
	return nil
}
