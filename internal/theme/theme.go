// Package theme provides a color theme for terminal output.
package theme

import (
	"fmt"
	"os"
	"regexp"

	"github.com/IgorBolotnikov/DRUDGE/internal/common"
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

// Theme holds the effective foreground color of each role after all merges.
// It is immutable after creation.
type Theme struct {
	colors      map[string]color
	isColorless bool
}

// Environment variables that decide whether color is on. See
// https://no-color.org and https://bixense.com/clicolors.
const (
	noColorEnv       = "NO_COLOR"
	forceColorEnv    = "FORCE_COLOR"
	cliColorForceEnv = "CLICOLOR_FORCE"
	cliColorEnv      = "CLICOLOR"
	termEnv          = "TERM"
)

// Values of the color environment variables that turn color off.
const (
	colorOffValue = "0"
	dumbTerm      = "dumb"
)

// isColorlessEnv decides whether color is off. The order of the cases is the
// precedence of the rules, and the first case that matches wins.
func isColorlessEnv() bool {
	switch {
	case os.Getenv(noColorEnv) != "":
		return true
	case isForcedEnv(forceColorEnv) || isForcedEnv(cliColorForceEnv):
		return false
	case os.Getenv(cliColorEnv) == colorOffValue:
		return true
	case os.Getenv(termEnv) == dumbTerm:
		return true
	default:
		return !common.IsTerminal(os.Stdout)
	}
}

func isForcedEnv(name string) bool {
	value := os.Getenv(name)
	return value != "" && value != colorOffValue
}

// ansiReset is the ANSI reset sequence.
const ansiReset = "\x1b[0m"

// ansiColorPrefix is the ANSI 24-bit true color prefix.
const ansiColorPrefix = "\x1b[38;2;%d;%d;%dm"

// defaultTheme is the fallback theme when none is configured.
const defaultTheme = systemTheme

// DefaultTheme returns the name of the fallback theme.
func DefaultTheme() string {
	return defaultTheme
}

// ThemeSchemaRef returns the $schema reference path for theme.json.
func ThemeSchemaRef() string {
	return themeSchemaRef
}

// NewTheme creates a Theme from a bundled palette name.
func NewTheme(name string) *Theme {
	colors, ok := bundledColors(name)
	if !ok {
		colors = make(map[string]color)
	}
	return &Theme{
		colors:      colors,
		isColorless: isColorlessEnv(),
	}
}

// Color returns the ANSI escape sequence of the color of the given role. It
// returns an empty string when color is off or the role is unknown.
func (t *Theme) Color(role string) string {
	if t.isColorless {
		return ""
	}
	roleColor, ok := t.colors[role]
	if !ok {
		return ""
	}
	return roleColor.escape()
}

// Reset returns the ANSI reset sequence, or an empty string when color is
// off.
func (t *Theme) Reset() string {
	if t.isColorless {
		return ""
	}
	return ansiReset
}

// Hex returns the raw "#rrggbb" string for the given role, or an empty string
// when the role is unknown or its color is not a hex color.
func (t *Theme) Hex(role string) string {
	hex, _ := t.colors[role].(hexColor)
	return string(hex)
}

// Colors returns the "#rrggbb" string of every role with a hex color.
func (t *Theme) Colors() map[string]string {
	hexes := make(map[string]string, len(t.colors))
	for role, roleColor := range t.colors {
		if hex, ok := roleColor.(hexColor); ok {
			hexes[role] = string(hex)
		}
	}
	return hexes
}

// themeSchemaRef is the $schema reference path in theme.json.
const themeSchemaRef = "./schema/theme.json"

// validHex checks whether s is a valid 24-bit hex color (#rrggbb).
var hexPattern = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

// config represents the structure of theme file.
type config struct {
	Theme     string            `json:"theme"`
	Overrides map[string]string `json:"overrides"`
}

func validHex(s string) bool {
	return hexPattern.MatchString(s)
}

// Load constructs a Theme by loading the bundled palette, applying overrides
// from the theme file, validating all color values, and returning the result.
// If name is empty, falls back to the theme in the config file, then defaultTheme.
func Load(name string) (*Theme, error) {
	home, err := common.HomeDir()
	if err != nil {
		return nil, fmt.Errorf("could not determine home directory: %w", err)
	}

	cfgPath := common.ThemeConfigPath(home)
	var cfg config

	isPresent, statErr := common.Exists(cfgPath)
	if statErr != nil {
		return nil, fmt.Errorf("could not read theme config: %w", statErr)
	}

	if isPresent {
		if err := common.ReadJSON(cfgPath, &cfg); err != nil {
			return nil, fmt.Errorf("could not parse theme config: %w", err)
		}
	}

	if cfg.Overrides == nil {
		cfg.Overrides = map[string]string{}
	}

	paletteName := name
	if paletteName == "" && cfg.Theme != "" {
		paletteName = cfg.Theme
	}
	if paletteName == "" {
		paletteName = defaultTheme
	}

	merged, ok := bundledColors(paletteName)
	if !ok {
		return nil, fmt.Errorf("unknown theme %q", paletteName)
	}

	logger := common.NewLogger("theme")
	for role, color := range cfg.Overrides {
		if !validHex(color) {
			logger.Info("invalid hex color %q for role %q, falling back to palette default", color, role)
			continue
		}
		merged[role] = hexColor(color)
	}

	return &Theme{colors: merged, isColorless: isColorlessEnv()}, nil
}

// MustLoad is like Load but panics on error.
func MustLoad() *Theme {
	th, err := Load("")
	if err != nil {
		panic(err)
	}
	return th
}
