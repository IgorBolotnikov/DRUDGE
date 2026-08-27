package main

import (
	"fmt"
	"os"

	"drudge/internal/cmd"
)

func main() {
	cli := cmd.NewCLI()

	cli.Register(
		cmd.InitCmd,
		cmd.SetupCmd,
		cmd.CleanupCmd,
		cmd.ProjectCmd,
	)

	if err := cli.Run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}
