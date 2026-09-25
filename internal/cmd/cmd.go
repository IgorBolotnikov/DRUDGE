// Package cmd with CLI commands
package cmd

import (
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/IgorBolotnikov/DRUDGE/internal/theme"
)

type Cmd struct {
	Name  string
	Usage string
	Desc  string
	Run   func(args []string) error
}

type CLI struct {
	Cmds    map[string]*Cmd
	Version string
}

func NewCLI(version string) *CLI {
	return &CLI{Cmds: make(map[string]*Cmd), Version: version}
}

func (c *CLI) Register(cmds ...*Cmd) {
	for _, cmd := range cmds {
		c.Cmds[cmd.Name] = cmd
	}
}

func (c *CLI) Run(args []string) error {
	if len(args) < 1 || args[0] == helpFlag || args[0] == helpFlagShort {
		c.printHelp()
		return nil
	}
	if args[0] == versionFlag {
		fmt.Printf("drg %s\n", c.Version)
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
	fmt.Println("Available commands:")
	names := make([]string, 0, len(c.Cmds))
	for name := range c.Cmds {
		names = append(names, name)
	}
	slices.Sort(names)
	for _, name := range names {
		fmt.Printf("  %-12s %s\n", name, c.Cmds[name].Desc)
	}
}

var logoLines = []string{
	"  ██████╗ ██████╗ ██╗   ██╗██████╗  ██████╗ ███████╗",
	"  ██╔══██╗██╔══██╗██║   ██║██╔══██╗██╔════╝ ██╔════╝",
	"  ██║  ██║██████╔╝██║   ██║██║  ██║██║  ███╗█████╗",
	"  ██║  ██║██╔══██╗██║   ██║██║  ██║██║   ██║██╔══╝",
	"  ██████╔╝██║  ██║╚██████╔╝██████╔╝╚██████╔╝███████╗",
	"  ╚═════╝ ╚═╝  ╚═╝ ╚═════╝ ╚═════╝  ╚═════╝ ╚══════╝",
}

// logoFaceChar is the character of the letter faces. Every other character
// of the logo is the outline.
const logoFaceChar = '█'

// The letter faces fade linearly from logoFaceTopShift in the top row to
// logoFaceBottomShift in the bottom row. Both shift the HSV value of the error
// color.
const (
	logoFaceTopShift    = 0.25
	logoFaceBottomShift = -0.25
	logoOutlineShift    = -0.3
)

// printProjectName prints the logo when stdout is a terminal.
func printProjectName() {
	if !isTerminal(os.Stdout) {
		return
	}
	th := theme.MustLoad()
	outlineColor := th.Shade(theme.RoleError, logoOutlineShift)
	fmt.Println("")
	for rowIndex, line := range logoLines {
		progress := float64(rowIndex) / float64(len(logoLines)-1)
		faceShift := logoFaceTopShift + (logoFaceBottomShift-logoFaceTopShift)*progress
		faceColor := th.Shade(theme.RoleError, faceShift)
		var builder strings.Builder
		currentColor := ""
		for _, char := range line {
			charColor := outlineColor
			if char == logoFaceChar {
				charColor = faceColor
			}
			if charColor != currentColor {
				builder.WriteString(charColor)
				currentColor = charColor
			}
			builder.WriteRune(char)
		}
		fmt.Println(builder.String() + th.Reset())
	}
	fmt.Println("")
}

func isTerminal(file *os.File) bool {
	info, err := file.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
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
