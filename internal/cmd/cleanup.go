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

		if !HasForceFlag(args) {
			confirmed, err := ConfirmDeletion(drudgeDir)
			if err != nil {
				return err
			}
			if !confirmed {
				fmt.Println("Aborted")
				return nil
			}
		}

		if err := common.RemoveAll(drudgeDir); err != nil {
			return err
		}

		fmt.Printf("Removed %s\n", drudgeDir)
		return nil
	},
}
