package theme

import "fmt"

// color is the color of one role in a theme.
type color interface {
	// escape returns the ANSI escape sequence that sets the foreground to the
	// color.
	escape() string
	// rgb returns the red, green and blue bytes of the color. ok is false for
	// a color that has no RGB.
	rgb() (red, green, blue int, ok bool)
}

// hexColor is a 24-bit color written as "#rrggbb".
type hexColor string

func (hex hexColor) escape() string {
	red, green, blue := hexToRGB(string(hex))
	return rgbEscape(red, green, blue)
}

func (hex hexColor) rgb() (red, green, blue int, ok bool) {
	red, green, blue = hexToRGB(string(hex))
	return red, green, blue, true
}

// ansiColor is one of the 16 basic ANSI colors, stored as its SGR code. The
// terminal theme of the user decides how it looks, so it has no RGB.
type ansiColor int

// SGR codes of the basic ANSI colors used by the system theme.
const (
	ansiRed         ansiColor = 31
	ansiGreen       ansiColor = 32
	ansiYellow      ansiColor = 33
	ansiBlue        ansiColor = 34
	ansiCyan        ansiColor = 36
	ansiWhite       ansiColor = 37
	ansiBrightBlack ansiColor = 90
)

// ansiSGRFormat is the ANSI escape sequence that sets an SGR code.
const ansiSGRFormat = "\x1b[%dm"

func (sgr ansiColor) escape() string {
	return fmt.Sprintf(ansiSGRFormat, int(sgr))
}

func (sgr ansiColor) rgb() (red, green, blue int, ok bool) {
	return 0, 0, 0, false
}

// rgbEscape returns the ANSI 24-bit true color escape sequence.
func rgbEscape(red, green, blue int) string {
	return fmt.Sprintf(ansiColorPrefix, red, green, blue)
}

// hexToRGB parses a "#rrggbb" string and returns the red, green, and blue
// byte values as ints.
func hexToRGB(hex string) (int, int, int) {
	var r, g, b int
	fmt.Sscanf(hex, "#%02x%02x%02x", &r, &g, &b)
	return r, g, b
}

// hexColors converts a palette of "#rrggbb" strings into colors.
func hexColors(palette map[string]string) map[string]color {
	colors := make(map[string]color, len(palette))
	for role, hex := range palette {
		colors[role] = hexColor(hex)
	}
	return colors
}
