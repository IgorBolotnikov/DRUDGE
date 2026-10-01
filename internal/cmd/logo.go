package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/IgorBolotnikov/DRUDGE/internal/common"
	"github.com/IgorBolotnikov/DRUDGE/internal/theme"
)

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

// printProjectName prints the logo in the colors of th when stdout is a
// terminal.
func printProjectName(th *theme.Theme) {
	if !common.IsTerminal(os.Stdout) {
		return
	}
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
