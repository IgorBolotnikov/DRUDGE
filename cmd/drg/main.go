package main

import (
	"fmt"
	"os"
	"runtime/debug"

	"github.com/IgorBolotnikov/DRUDGE/internal/cmd"
)

// version is stamped by the release build with -ldflags "-X main.version=...".
var version = ""

func main() {
	version := resolveVersion()
	cli := cmd.NewCLI(version)

	cli.Register(
		cmd.InitCmd,
		cmd.SetupCmd,
		cmd.CleanupCmd,
		cmd.ProjectCmd,
		cmd.TaskCmd,
		cmd.DrudgerCmd,
		cmd.NewUpdateCmd(version),
	)

	if err := cli.Run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}

// resolveVersion falls back to the module version that go install records,
// and to "dev" for a local build.
func resolveVersion() string {
	if version != "" {
		return version
	}
	info, ok := debug.ReadBuildInfo()
	if ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}
	return "dev"
}
