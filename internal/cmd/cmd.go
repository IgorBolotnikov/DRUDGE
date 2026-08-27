// Package cmd with CLI commands
package cmd

import (
	"fmt"
)

var (
	ErrNoProjectName = fmt.Errorf("project name is required, usage: drg project create <name>")
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
	printProjectName()
	fmt.Println("\nAvailable commands:")
	for _, cmd := range c.Cmds {
		fmt.Printf("  %-12s %s\n", cmd.Name, cmd.Desc)
	}
}

func printProjectName() {
	fmt.Println(`
 ██████╗ ██████╗ ██╗   ██╗██████╗  ██████╗ ███████╗
 ██╔══██╗██╔══██╗██║   ██║██╔══██╗██╔════╝ ██╔════╝
 ██║  ██║██████╔╝██║   ██║██║  ██║██║  ███╗█████╗
 ██║  ██║██╔══██╗██║   ██║██║  ██║██║   ██║██╔══╝
 ██████╔╝██║  ██║╚██████╔╝██████╔╝╚██████╔╝███████╗
 ╚═════╝ ╚═╝  ╚═╝ ╚═════╝ ╚═════╝  ╚═════╝ ╚══════╝`)
}
