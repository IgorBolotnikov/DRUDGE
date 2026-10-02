package cmd

import (
	"flag"

	"github.com/IgorBolotnikov/DRUDGE/internal/adapters/remote"
	"github.com/IgorBolotnikov/DRUDGE/internal/release"
)

// NewUpdateCmd returns the update command for a drg binary of version.
func NewUpdateCmd(version string) *Cmd {
	return &Cmd{
		Name: "update",
		Desc: "Update drg to the latest release",
		Setup: func(*flag.FlagSet) func(args []string) error {
			return func([]string) error { return update(version) }
		},
	}
}

func update(version string) error {
	binaryPath, err := release.ExecutablePath()
	if err != nil {
		return err
	}
	repo := remote.NewReleases(remote.RepositoryURL, remote.RequestTimeout)
	out := newCommandPrinter()
	service := release.NewReleaseService(repo, newCLIProgress(out))

	result, err := service.Update(version, binaryPath)
	if err != nil {
		return err
	}
	if result.IsUpToDate {
		out.Skip("drg %s is the latest release", result.Version)
		return nil
	}
	out.Result("Updated drg %s to %s", result.PreviousVersion, result.Version)
	out.Field("Binary", result.BinaryPath)
	out.Field("Next", "run drg setup to refresh the schema files and the skill")
	out.Flush()
	return nil
}
