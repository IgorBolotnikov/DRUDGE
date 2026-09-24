// Package cmd with CLI commands
package cmd

import (
	"fmt"

	"drudge/internal/theme"
)

type Cmd struct {
	Name  string
	Usage string
	Desc  string
	Run   func(args []string) error
}

type CLI struct {
	Cmds map[string]*Cmd
}

func NewCLI() *CLI {
	return &CLI{Cmds: make(map[string]*Cmd)}
}

func (c *CLI) Register(cmds ...*Cmd) {
	for _, cmd := range cmds {
		c.Cmds[cmd.Name] = cmd
	}
}

func (c *CLI) Run(args []string) error {
	if len(args) < 1 {
		c.printHelp()
		return nil
	}

	cmd, ok := c.Cmds[args[0]]
	if !ok {
		return fmt.Errorf("unknown command: %s", args[0])
	}

	return cmd.Run(args[1:])
}

func (c *CLI) printHelp() {
	fmt.Println("")
	printProjectName()
	fmt.Println("\nAvailable commands:")
	for _, cmd := range c.Cmds {
		fmt.Printf("  %-12s %s\n", cmd.Name, cmd.Desc)
	}
}

func printProjectName() {
	lines := []string{
		"  ██████╗ ██████╗ ██╗   ██╗██████╗  ██████╗ ███████╗",
		"  ██╔══██╗██╔══██╗██║   ██║██╔══██╗██╔════╝ ██╔════╝",
		"  ██║  ██║██████╔╝██║   ██║██║  ██║██║  ███╗█████╗",
		"  ██║  ██║██╔══██╗██║   ██║██║  ██║██║   ██║██╔══╝",
		"  ██████╔╝██║  ██║╚██████╔╝██████╔╝╚██████╔╝███████╗",
		"  ╚═════╝ ╚═╝  ╚═╝ ╚═════╝ ╚═════╝  ╚═════╝ ╚══════╝",
	}
	th := theme.MustLoad()
	errColor := th.Color(theme.RoleError)
	for _, line := range lines {
		fmt.Println(errColor + line + th.Reset())
	}
}

// HasForceFlag reports whether a slice of args contains --force or -f.
func HasForceFlag(args []string) bool {
	for _, a := range args {
		if a == "--force" || a == "-f" {
			return true
		}
	}
	return false
}

// ConfirmDeletion asks the user to confirm deleting a resource and reports
// what they answered. Anything but y or Y calls the deletion off.
func ConfirmDeletion(resource string) (isConfirmed bool, err error) {
	fmt.Printf("This will permanently delete %s\nAre you sure? [y/N]: ", resource)
	var response string
	if _, err := fmt.Scanln(&response); err != nil && err.Error() != "unexpected newline" {
		return false, fmt.Errorf("could not read confirmation: %w", err)
	}
	return response == "y" || response == "Y", nil
}
