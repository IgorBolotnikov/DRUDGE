// Package theme provides a color theme for terminal output.
package theme

import (
	"fmt"
	"maps"
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

// Theme holds the effective color palette (foreground only, 24-bit hex) after
// all merges. It is immutable after creation.
type Theme struct {
	colors      map[string]string // role -> "#rrggbb"
	isColorless bool
}

// noColorEnv turns off every escape sequence when set to a non-empty value.
// See https://no-color.org.
const noColorEnv = "NO_COLOR"

func isColorlessEnv() bool {
	return os.Getenv(noColorEnv) != ""
}

// ansiReset is the ANSI reset sequence.
const ansiReset = "\x1b[0m"

// ansiColorPrefix is the ANSI 24-bit true color prefix.
const ansiColorPrefix = "\x1b[38;2;%d;%d;%dm"

// defaultTheme is the fallback theme when none is configured.
const defaultTheme = "nord"

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
	palette, ok := bundledPalettes[name]
	if !ok {
		palette = make(map[string]string)
	}
	return &Theme{
		colors:      copyMap(palette),
		isColorless: isColorlessEnv(),
	}
}

// Color returns a 24-bit true color ANSI escape sequence for the given role.
// Format: \x1b[38;2;R;G;mb. It returns an empty string when NO_COLOR is set.
func (t *Theme) Color(role string) string {
	if t.isColorless {
		return ""
	}
	hex, ok := t.colors[role]
	if !ok {
		return ""
	}
	r, g, b := hexToRGB(hex)
	return fmt.Sprintf(ansiColorPrefix, r, g, b)
}

// Reset returns the ANSI reset sequence, or an empty string when NO_COLOR is
// set.
func (t *Theme) Reset() string {
	if t.isColorless {
		return ""
	}
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

// hexToRGB parses a "#rrggbb" string and returns the red, green, and blue
// byte values as ints.
func hexToRGB(hex string) (int, int, int) {
	var r, g, b int
	fmt.Sscanf(hex, "#%02x%02x%02x", &r, &g, &b)
	return r, g, b
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

	palette, ok := bundledPalettes[paletteName]
	if !ok {
		return nil, fmt.Errorf("unknown theme %q", paletteName)
	}

	merged := copyMap(palette)
	logger := common.NewLogger("theme")
	for role, color := range cfg.Overrides {
		if !validHex(color) {
			logger.Info("invalid hex color %q for role %q, falling back to palette default", color, role)
			continue
		}
		merged[role] = color
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
