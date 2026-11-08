package main

import (
	"os"
	"runtime/debug"

	"github.com/IgorBolotnikov/DRUDGE/internal/ui/cli"
)

// version is stamped by the release build with -ldflags "-X main.version=...".
var version = ""

func main() {
	version := resolveVersion()
	root := cli.NewRoot(version)
	root.Validate()

	if err := root.Execute(os.Args[1:]); err != nil {
		cli.NewLogger().Error("%v", err)
		os.Exit(cli.ExitCode(err))
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
