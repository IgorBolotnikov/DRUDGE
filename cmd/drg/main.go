package main

import (
	"fmt"
	"os"

	"github.com/IgorBolotnikov/DRUDGE/internal/cmd"
)

func main() {
	cli := cmd.NewCLI()

	cli.Register(
		cmd.InitCmd,
		cmd.SetupCmd,
		cmd.CleanupCmd,
		cmd.ProjectCmd,
		cmd.TaskCmd,
		cmd.DrudgerCmd,
	)

	if err := cli.Run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}
