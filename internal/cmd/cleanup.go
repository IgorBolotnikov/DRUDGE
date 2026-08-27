package cmd

import (
	"fmt"

	"drudge/internal/common"
)

var CleanupCmd = &Cmd{
	Name:  "cleanup",
	Usage: "cleanup",
	Desc:  "Cleanup DRUDGE from this computer",
	Run: func(args []string) error {
		home, err := common.HomeDir()
		if err != nil {
			return err
		}

		drudgeDir := common.DrudgeDir(home)

		exists, err := common.Exists(drudgeDir)
		if err != nil {
			return err
		}
		if !exists {
			fmt.Printf("Nothing to clean up, %s does not exist\n", drudgeDir)
			return nil
		}

		force := HasForceFlag(args)

		if err := ConfirmDeletion(drudgeDir, force); err != nil {
			return err
		}

		if err := common.RemoveAll(drudgeDir); err != nil {
			return err
		}

		fmt.Printf("Removed %s\n", drudgeDir)
		return nil
	},
}
