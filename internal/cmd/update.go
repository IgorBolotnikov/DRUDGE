package cmd

import (
	"fmt"

	"github.com/IgorBolotnikov/DRUDGE/internal/adapters/github"
	"github.com/IgorBolotnikov/DRUDGE/internal/common"
	"github.com/IgorBolotnikov/DRUDGE/internal/release"
)

// NewUpdateCmd returns the update command for a drg binary of version.
func NewUpdateCmd(version string) *Cmd {
	return &Cmd{
		Name:  "update",
		Usage: "update",
		Desc:  "Update drg to the latest release",
		Run: func(args []string) error {
			binaryPath, err := release.ExecutablePath()
			if err != nil {
				return err
			}
			repo := github.New(github.RepositoryURL, github.RequestTimeout)
			service := release.NewReleaseService(repo, common.NewLogger(""))

			result, err := service.Update(version, binaryPath)
			if err != nil {
				return err
			}
			if result.IsUpToDate {
				fmt.Printf("drg %s is the latest release\n", result.Version)
				return nil
			}
			fmt.Printf("Updated drg %s to %s at %s\n", result.PreviousVersion, result.Version, result.BinaryPath)
			fmt.Println("Run drg setup to refresh the schema files and the skill")
			return nil
		},
	}
}
