// Package theme provides a color theme for terminal output.
package theme

import (
	"fmt"
	"maps"
)

// Canonical roles in the theme palette.
const (
	RolePrimary   = "primary"
	RoleHeading   = "heading"
	RoleSuccess   = "success"
	RoleError     = "error"
	RoleWarning   = "warning"
	RoleInfo      = "info"
	RoleMuted     = "muted"
	RoleSecondary = "secondary"
	RoleBorder    = "border"
	RolePath      = "path"
)

// Theme holds the effective color palette (foreground only, 24-bit hex) after
// all merges. It is immutable after creation.
type Theme struct {
	colors map[string]string // role -> "#rrggbb"
}

// ansiReset is the ANSI reset sequence.
const ansiReset = "\x1b[0m"

// NewTheme creates a Theme from a bundled palette name.
func NewTheme(name string) *Theme {
	palette, ok := bundledPalettes[name]
	if !ok {
		palette = make(map[string]string)
	}
	return &Theme{
		colors: copyMap(palette),
	}
}

// ANSI returns a 24-bit true color ANSI escape sequence for the given role.
// Format: \x1b[38;2;R;G;mb\x1b[0m.
func (t *Theme) ANSI(role string) string {
	hex, ok := t.colors[role]
	if !ok {
		return ansiReset
	}
	return hexToANSI(hex)
}

// Reset returns the ANSI reset sequence.
func (t *Theme) Reset() string {
	return ansiReset
}

// Hex returns the raw "#rrggbb" string for the given role.
func (t *Theme) Hex(role string) string {
	return t.colors[role]
}

// Colors returns a copy of the effective palette map after all merges.
func (t *Theme) Colors() map[string]string {
	return copyMap(t.colors)
}

// copyMap returns a shallow copy of the given map.
func copyMap(m map[string]string) map[string]string {
	out := make(map[string]string, len(m))
	maps.Copy(out, m)
	return out
}

// hexToANSI converts a "#rrggbb" hex string to a 24-bit true color ANSI
// escape sequence: \x1b[38;2;R;G;mb\x1b[0m.
func hexToANSI(hex string) string {
	r, g, b := hexToRGB(hex)
	return fmt.Sprintf("\x1b[38;2;%d;%d;%dm\x1b[0m", r, g, b)
}

// hexToRGB parses a "#rrggbb" string and returns the red, green, and blue
// byte values as ints.
func hexToRGB(hex string) (int, int, int) {
	var r, g, b int
	fmt.Sscanf(hex, "#%02x%02x%02x", &r, &g, &b)
	return r, g, b
}
