package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"drudge/internal/common"
)

var CleanupCmd = &Cmd{
	Name:  "cleanup",
	Usage: "cleanup",
	Desc:  "Cleanup DRUDGE from this computer",
	Run: func(args []string) error {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("could not determine home directory: %w", err)
		}

		drudgeDir := filepath.Join(home, ".drudge")

		exists, err := common.Exists(drudgeDir)
		if err != nil {
			return err
		}
		if !exists {
			fmt.Printf("Nothing to clean up, %s does not exist\n", drudgeDir)
			return nil
		}

		force := false
		for _, a := range args {
			if a == "--force" || a == "-f" {
				force = true
			}
		}

		if !force {
			fmt.Printf("This will permanently delete %s\nAre you sure? [y/N]: ", drudgeDir)
			var response string
			if _, err := fmt.Scanln(&response); err != nil && err.Error() != "unexpected newline" {
				return fmt.Errorf("could not read confirmation: %w", err)
			}
			if response != "y" && response != "Y" {
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
