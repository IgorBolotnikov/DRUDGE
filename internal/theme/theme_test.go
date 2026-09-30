package theme

import (
	"maps"
	"os"
	"testing"

	"github.com/IgorBolotnikov/DRUDGE/internal/common"
)

func TestBundledThemesHaveAllRoles(t *testing.T) {
	for name := range bundledPalettes {
		t.Run(name, func(t *testing.T) {
			th := NewTheme(name)
			for _, role := range allRoles() {
				if th.Hex(role) == "" {
					t.Errorf("theme %q missing role %q", name, role)
				}
			}
		})
	}
}

func TestColorShadeAndReset_FollowColorEnv(t *testing.T) {
	const wantColor = "\x1b[38;2;191;97;106m"
	cases := []struct {
		name      string
		env       map[string]string
		isColorOn bool
	}{
		{name: "stdout is not a terminal", env: map[string]string{}, isColorOn: false},
		{name: "NO_COLOR set", env: map[string]string{noColorEnv: "1"}, isColorOn: false},
		{name: "FORCE_COLOR set", env: map[string]string{forceColorEnv: "1"}, isColorOn: true},
		{name: "CLICOLOR_FORCE set", env: map[string]string{cliColorForceEnv: "1"}, isColorOn: true},
		{name: "FORCE_COLOR=0 does not force", env: map[string]string{forceColorEnv: "0"}, isColorOn: false},
		{name: "CLICOLOR_FORCE=0 does not force", env: map[string]string{cliColorForceEnv: "0"}, isColorOn: false},
		{name: "CLICOLOR=0", env: map[string]string{cliColorEnv: "0"}, isColorOn: false},
		{name: "TERM=dumb", env: map[string]string{termEnv: "dumb"}, isColorOn: false},
		{name: "NO_COLOR wins over FORCE_COLOR", env: map[string]string{noColorEnv: "1", forceColorEnv: "1"}, isColorOn: false},
		{name: "NO_COLOR wins over CLICOLOR_FORCE", env: map[string]string{noColorEnv: "1", cliColorForceEnv: "1"}, isColorOn: false},
		{name: "FORCE_COLOR wins over CLICOLOR=0", env: map[string]string{forceColorEnv: "1", cliColorEnv: "0"}, isColorOn: true},
		{name: "CLICOLOR_FORCE wins over CLICOLOR=0", env: map[string]string{cliColorForceEnv: "1", cliColorEnv: "0"}, isColorOn: true},
		{name: "FORCE_COLOR wins over TERM=dumb", env: map[string]string{forceColorEnv: "1", termEnv: "dumb"}, isColorOn: true},
		{name: "CLICOLOR_FORCE wins over TERM=dumb", env: map[string]string{cliColorForceEnv: "1", termEnv: "dumb"}, isColorOn: true},
		{name: "FORCE_COLOR=0 does not win over CLICOLOR_FORCE", env: map[string]string{forceColorEnv: "0", cliColorForceEnv: "1"}, isColorOn: true},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			for _, name := range []string{noColorEnv, forceColorEnv, cliColorForceEnv, cliColorEnv, termEnv} {
				t.Setenv(name, testCase.env[name])
			}
			setupTempHome(t, `{"theme": "nord"}`)
			themes := map[string]*Theme{"NewTheme": NewTheme("nord"), "Load": MustLoad()}

			want := map[string]string{"Color": "", "Shade": "", "Reset": ""}
			if testCase.isColorOn {
				want = map[string]string{"Color": wantColor, "Shade": wantColor, "Reset": ansiReset}
			}
			for constructor, theme := range themes {
				got := map[string]string{
					"Color": theme.Color(RoleError),
					"Shade": theme.Shade(RoleError, 0),
					"Reset": theme.Reset(),
				}
				for method, wantValue := range want {
					if got[method] != wantValue {
						t.Errorf("%s: %s() = %q, want %q", constructor, method, got[method], wantValue)
					}
				}
			}
		})
	}
}

func TestIsColorOn_FollowsColorEnvOnEveryStream(t *testing.T) {
	cases := []struct {
		name      string
		env       map[string]string
		isColorOn bool
	}{
		{name: "NO_COLOR set", env: map[string]string{noColorEnv: "1"}, isColorOn: false},
		{name: "FORCE_COLOR set", env: map[string]string{forceColorEnv: "1"}, isColorOn: true},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			for _, name := range []string{noColorEnv, forceColorEnv, cliColorForceEnv, cliColorEnv, termEnv} {
				t.Setenv(name, testCase.env[name])
			}
			theme := NewTheme("nord")
			for stream, streamName := range map[Stream]string{Stdout: "Stdout", Stderr: "Stderr"} {
				if got := theme.IsColorOn(stream); got != testCase.isColorOn {
					t.Errorf("IsColorOn(%s) = %v, want %v", streamName, got, testCase.isColorOn)
				}
			}
		})
	}
}

func TestHex_ExactValue(t *testing.T) {
	th := NewTheme("dracula")
	tests := []struct {
		role   string
		expect string
	}{
		{"primary", "#BD93F9"},
		{"error", "#FF5555"},
		{"success", "#50FA7B"},
		{"heading", "#FF79C6"},
		{"muted", "#6272A4"},
	}
	for _, tt := range tests {
		t.Run(tt.role, func(t *testing.T) {
			got := th.Hex(tt.role)
			if got != tt.expect {
				t.Errorf("Hex(%q) = %q, want %q", tt.role, got, tt.expect)
			}
		})
	}
}

func TestColors_ReturnsCopy(t *testing.T) {
	th := NewTheme("nord")
	c1 := th.Colors()
	c1["primary"] = "#000000"
	c2 := th.Colors()
	if c2["primary"] == "#000000" {
		t.Error("Colors() should return a copy; modifying the result should not affect the theme")
	}
}

func TestColors_NotSameReference(t *testing.T) {
	th := NewTheme("nord")
	c1 := th.Colors()
	c1["newrole"] = "#123456"
	c2 := th.Colors()
	if _, ok := c2["newrole"]; ok {
		t.Error("Colors() should return a copy; new keys should not leak between calls")
	}
}

func TestNewTheme_UnknownName(t *testing.T) {
	th := NewTheme("nonexistent-theme")
	for _, role := range allRoles() {
		if th.Hex(role) != "" {
			t.Errorf("unknown theme should have empty colors, but Hex(%q) = %q", role, th.Hex(role))
		}
	}
}

func TestLoad_DefaultSystem(t *testing.T) {
	cases := []struct {
		name      string
		themeFile string
	}{
		{name: "no theme file", themeFile: ""},
		{name: "empty theme file", themeFile: "{}"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Setenv(noColorEnv, "")
			t.Setenv(forceColorEnv, "1")
			setupTempHome(t, testCase.themeFile)

			theme, err := Load("")
			if err != nil {
				t.Fatalf("Load: %v", err)
			}

			system := NewTheme(systemTheme)
			for _, role := range allRoles() {
				if got, want := theme.Color(role), system.Color(role); got != want {
					t.Errorf("Color(%q) = %q, want %q", role, got, want)
				}
			}
		})
	}
}

func TestLoad_DefaultCreatesNoThemeFile(t *testing.T) {
	home := setupTempHome(t, "")

	if _, err := Load(""); err != nil {
		t.Fatalf("Load: %v", err)
	}

	_, err := os.Stat(common.ThemeConfigPath(home))
	if !os.IsNotExist(err) {
		t.Error("theme.json should not be created")
	}
}

func TestLoad_CatppuccinTheme(t *testing.T) {
	setupTempHome(t, `{"theme":"catppuccin-mocha"}`)

	th, err := Load("catppuccin-mocha")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	for _, role := range allRoles() {
		if th.Hex(role) != catppuccinMochaPalette[role] {
			t.Errorf("Load() catppuccin-mocha %q = %q, want %q", role, th.Hex(role), catppuccinMochaPalette[role])
		}
	}
}

func TestLoad_OverridesMerge(t *testing.T) {
	setupTempHome(t, `{"theme":"nord","overrides":{"error":"#ff0000","success":"#00ff00"}}`)

	th, err := Load("nord")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if th.Hex("error") != "#ff0000" {
		t.Errorf("expected error override #ff0000, got %q", th.Hex("error"))
	}
	if th.Hex("success") != "#00ff00" {
		t.Errorf("expected success override #00ff00, got %q", th.Hex("success"))
	}
	if th.Hex("primary") != nordPalette["primary"] {
		t.Errorf("unrelated role primary should be unchanged, got %q", th.Hex("primary"))
	}
}

func TestLoad_UnknownTheme(t *testing.T) {
	_, err := Load("nonexistent")
	if err == nil {
		t.Fatal("expected error for unknown theme")
	}
	expected := `unknown theme "nonexistent"`
	if err.Error() != expected {
		t.Errorf("expected %q, got %q", expected, err.Error())
	}
}

func TestLoad_InvalidHexFallback(t *testing.T) {
	setupTempHome(t, `{"theme":"nord","overrides":{"error":"#GGGGGG"}}`)

	th, err := Load("nord")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if th.Hex("error") != nordPalette["error"] {
		t.Errorf("invalid hex should fall back to palette default, got %q", th.Hex("error"))
	}
}

func TestLoad_AllBundledThemes(t *testing.T) {
	names := []string{"nord", "monokai", "catppuccin-mocha", "dracula"}
	palettes := map[string]map[string]string{
		"nord":             nordPalette,
		"monokai":          monokaiPalette,
		"catppuccin-mocha": catppuccinMochaPalette,
		"dracula":          draculaPalette,
	}

	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			setupTempHome(t, "")

			th, err := Load(name)
			if err != nil {
				t.Fatalf("Load: %v", err)
			}

			for _, role := range allRoles() {
				hex := th.Hex(role)
				if hex == "" {
					t.Errorf("Load(%q) missing role %q", name, role)
				}
				if !validHex(hex) {
					t.Errorf("Load(%q) role %q = %q is not a valid hex color", name, role, hex)
				}
				if hex != palettes[name][role] {
					t.Errorf("Load(%q) %q = %q, want %q", name, role, hex, palettes[name][role])
				}
			}
		})
	}
}

func TestLoad_Immutability(t *testing.T) {
	setupTempHome(t, "")

	th, err := Load("nord")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	original := th.Hex("primary")

	cols := th.Colors()
	cols["primary"] = "#000000"
	cols["newrole"] = "#123456"

	if th.Hex("primary") != original {
		t.Error("modifying Colors() result should not affect the theme")
	}

	if th.Hex("newrole") != "" {
		t.Error("new role added to Colors() result should not leak into theme")
	}
}

func TestLoad_MalformedJSON(t *testing.T) {
	setupTempHome(t, `{not json}`)

	_, err := Load("nord")
	if err == nil {
		t.Fatal("expected error for malformed JSON")
	}
}

func TestLoad_MultipleOverrides(t *testing.T) {
	setupTempHome(t, `{"theme":"nord","overrides":{"error":"#ff0000","success":"#00ff00","warning":"#ffff00","info":"#0000ff","primary":"#ffffff"}}`)

	th, err := Load("nord")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	expected := map[string]string{
		"error":   "#ff0000",
		"success": "#00ff00",
		"warning": "#ffff00",
		"info":    "#0000ff",
		"primary": "#ffffff",
	}

	for role, want := range expected {
		if th.Hex(role) != want {
			t.Errorf("Load() %q = %q, want %q", role, th.Hex(role), want)
		}
	}

	for _, role := range allRoles() {
		if _, ok := expected[role]; !ok {
			if th.Hex(role) != nordPalette[role] {
				t.Errorf("unmodified role %q should be palette default, got %q", role, th.Hex(role))
			}
		}
	}
}

func TestLoad_InvalidHexMixedWithValid(t *testing.T) {
	setupTempHome(t, `{"theme":"nord","overrides":{"error":"#GGGGGG","success":"#00ff00"}}`)

	th, err := Load("nord")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if th.Hex("error") != nordPalette["error"] {
		t.Errorf("invalid hex error should fall back, got %q", th.Hex("error"))
	}
	if th.Hex("success") != "#00ff00" {
		t.Errorf("valid hex success should be overridden, got %q", th.Hex("success"))
	}
}

func TestValidHex(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"#FFFFFF", true},
		{"#ff0000", true},
		{"#AaBbCc", true},
		{"#000000", true},
		{"#123456", true},
		{"#fff", false},
		{"ffffff", false},
		{"#GGGGGG", false},
		{"#1234567", false},
		{"#12345", false},
		{"#12345g", false},
		{"", false},
		{"#XYZXYZ", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := validHex(tt.input)
			if got != tt.want {
				t.Errorf("validHex(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func setupTempHome(t *testing.T, content string) string {
	t.Helper()

	home := t.TempDir()
	if err := os.Setenv("HOME", home); err != nil {
		t.Fatalf("Setenv: %v", err)
	}
	t.Cleanup(func() {
		os.Setenv("HOME", home)
	})

	if content != "" {
		if err := os.MkdirAll(common.DrudgeDir(home), 0o755); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}
		if err := os.WriteFile(common.ThemeConfigPath(home), []byte(content), common.DefaultFilePerm); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
	}

	return home
}

func allRoles() []string {
	return []string{
		"primary", "heading", "success", "error", "warning",
		"info", "muted", "secondary", "border", "path",
	}
}

func TestSystemTheme(t *testing.T) {
	wantColors := map[string]string{
		RolePrimary:   "\x1b[36m",
		RoleHeading:   "\x1b[34m",
		RoleSuccess:   "\x1b[32m",
		RoleError:     "\x1b[31m",
		RoleWarning:   "\x1b[33m",
		RoleInfo:      "\x1b[36m",
		RoleMuted:     "\x1b[90m",
		RoleSecondary: "\x1b[37m",
		RoleBorder:    "\x1b[90m",
		RolePath:      "\x1b[32m",
	}
	cases := []struct {
		name       string
		themeFile  string
		wantColors map[string]string
		wantHexes  map[string]string
	}{
		{
			name:       "every role is an ANSI color",
			themeFile:  `{"theme": "system"}`,
			wantColors: wantColors,
			wantHexes:  map[string]string{},
		},
		{
			name:       "a hex override replaces one role",
			themeFile:  `{"theme": "system", "overrides": {"error": "#ff0000"}}`,
			wantColors: withRole(wantColors, RoleError, "\x1b[38;2;255;0;0m"),
			wantHexes:  map[string]string{RoleError: "#ff0000"},
		},
		{
			name:       "an invalid override is skipped",
			themeFile:  `{"theme": "system", "overrides": {"error": "red"}}`,
			wantColors: wantColors,
			wantHexes:  map[string]string{},
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Setenv(noColorEnv, "")
			t.Setenv(forceColorEnv, "1")
			setupTempHome(t, testCase.themeFile)
			theme, err := Load("")
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			for role, want := range testCase.wantColors {
				if got := theme.Color(role); got != want {
					t.Errorf("Color(%q) = %q, want %q", role, got, want)
				}
				if got := theme.Hex(role); got != testCase.wantHexes[role] {
					t.Errorf("Hex(%q) = %q, want %q", role, got, testCase.wantHexes[role])
				}
			}
			if got := theme.Colors(); !maps.Equal(got, testCase.wantHexes) {
				t.Errorf("Colors() = %v, want %v", got, testCase.wantHexes)
			}
		})
	}

	t.Run("NewTheme", func(t *testing.T) {
		t.Setenv(noColorEnv, "")
		t.Setenv(forceColorEnv, "1")
		theme := NewTheme(systemTheme)
		for role, want := range wantColors {
			if got := theme.Color(role); got != want {
				t.Errorf("Color(%q) = %q, want %q", role, got, want)
			}
		}
	})
}

func TestSystemTheme_NoColor(t *testing.T) {
	t.Setenv(noColorEnv, "1")
	t.Setenv(forceColorEnv, "1")
	theme := NewTheme(systemTheme)
	got := map[string]string{
		"Color": theme.Color(RoleError),
		"Shade": theme.Shade(RoleError, 0.1),
		"Reset": theme.Reset(),
	}
	for method, value := range got {
		if value != "" {
			t.Errorf("%s() = %q, want empty", method, value)
		}
	}
}

func withRole(colors map[string]string, role, value string) map[string]string {
	result := maps.Clone(colors)
	result[role] = value
	return result
}
