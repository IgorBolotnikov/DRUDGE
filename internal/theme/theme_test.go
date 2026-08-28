package theme

import (
	"os"
	"path/filepath"
	"testing"

	"drudge/internal/common"
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

func TestReset(t *testing.T) {
	th := NewTheme("nord")
	got := th.Reset()
	expected := "\x1b[0m"
	if got != expected {
		t.Errorf("Reset() = %q, want %q", got, expected)
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

func TestLoad_DefaultNord(t *testing.T) {
	home := setupTempHome(t, "")

	th, err := Load("")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	for _, role := range allRoles() {
		if th.Hex(role) != nordPalette[role] {
			t.Errorf("Load() %q = %q, want %q", role, th.Hex(role), nordPalette[role])
		}
	}

	_, err = os.Stat(home + "/.drudge/theme.json")
	if !os.IsNotExist(err) {
		t.Error("theme.json should not be created")
	}
}

func TestLoad_EmptyConfig(t *testing.T) {
	setupTempHome(t, "{}")

	th, err := Load("")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	for _, role := range allRoles() {
		if th.Hex(role) != nordPalette[role] {
			t.Errorf("Load() with empty config %q = %q, want %q", role, th.Hex(role), nordPalette[role])
		}
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
		dir := filepath.Join(home, ".drudge")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}
		if err := os.WriteFile(filepath.Join(dir, ThemeConfigName), []byte(content), common.DefaultFilePerm); err != nil {
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
